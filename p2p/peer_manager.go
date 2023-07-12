package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"
	"torrent/bitfield"
)

const (
	handshakeLen = 68
	pstr         = "BitTorrent protocol"
)

type PeerManagers []*PeerManager

func (pms PeerManagers) FindByInfoHashAndIp(infoHash Hash, ip net.IP) PeerManagers {
	foundPms := make(PeerManagers, 0, 10)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash && pm.peer.IP.Equal(ip) {
			foundPms = append(foundPms, pm)
		}
	}
	return foundPms
}

func (pms PeerManagers) FindByInfoHash(infoHash Hash) PeerManagers {
	foundPms := make(PeerManagers, 0, 100)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash {
			foundPms = append(foundPms, pm)
		}
	}
	return foundPms
}

func (pms PeerManagers) FindAlive() PeerManagers {
	alivePms := make([]*PeerManager, 0, len(pms))
	for _, pm := range pms {
		if pm.IsAlive() {
			alivePms = append(alivePms, pm)
		}
	}
	return alivePms
}

type PeerManager struct {
	clientId         PeerID
	storage          StorageReader
	peer             Peer
	isAlive          atomic.Bool
	incomeMessagesCh chan *Message
	outcomeMessageCh chan *Message
	bitfield         []byte
	amChoking        atomic.Bool
	amInterested     atomic.Bool
	peerChoking      atomic.Bool
	peerInterested   atomic.Bool
	log              zerolog.Logger
	peerBitfield     *bitfield.Bitfield
	torrent          *TorrentFile
	exit             chan struct{}
	dms              map[Hash]*downloadManager
	dm               *downloadManager
	file             *os.File
}

func NewPeerManager(clientId PeerID, storage StorageReader, peer Peer, dms map[Hash]*downloadManager) *PeerManager {
	pm := &PeerManager{
		clientId:         clientId,
		storage:          storage,
		peer:             peer,
		incomeMessagesCh: make(chan *Message, 512),
		outcomeMessageCh: make(chan *Message, 512),
		dms:              dms,
	}
	pm.isAlive.Store(true)
	pm.amChoking.Store(true)
	pm.peerChoking.Store(true)
	return pm
}

func (pm *PeerManager) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		close(pm.outcomeMessageCh)
		if pm.peer.Conn != nil {
			pm.peer.Conn.Close()
		}
		pm.isAlive.Store(false)
	}()
	if !pm.isAlive.Load() {
		panic("peer manager is not alive")
	}
	pm.setLoggerFromCtx(ctx)

	var err error
	if pm.peer.Conn == nil {
		// we initiate connection and send a handshake first
		err = pm.sendHandshake(ctx)
		if err != nil {
			err = fmt.Errorf("unable to send handshake too remote peer: %w", err)
		}
	} else {
		// remote peer connected to us, and we are waiting for a handshake
		err = pm.acceptHandshake()
		if err != nil {
			err = fmt.Errorf("unable to accept handshake from remote peer: %w", err)
		}
	}
	if err != nil {
		pm.log.Error().Err(err).Send()
		return
	}
	pm.log.Info().Msg("successful handshake")

	pm.torrent = pm.storage.Get(pm.peer.InfoHash)
	pm.file = pm.storage.GetFile(pm.peer.InfoHash)
	pm.dm = pm.dms[pm.peer.InfoHash]

	_ = pm.sendMessage(NewBitfield(pm.storage.GetBitfield(pm.peer.InfoHash)))
	_ = pm.sendMessage(NewUnChoke())

	go pm.readMessages()
	go pm.writeMessages()
	go pm.handleMessages()
	go pm.download()

	err = pm.sendMessage(NewInterested())

	select {
	case <-ctx.Done():
	case <-pm.exit:
	}
}

func (pm *PeerManager) IsAlive() bool {
	return pm.isAlive.Load()
}

func (pm *PeerManager) sendHandshake(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	myHs := newHandshake(pm.peer.InfoHash, pm.clientId)
	dialer := &net.Dialer{
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", pm.peer.Address())
	if err != nil {
		return fmt.Errorf("unable to establish conn: %w", err)
	}
	defer conn.SetDeadline(time.Time{})
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	_, err = conn.Write(myHs.encode())
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to write handshake: %w", err)
	}
	buf := make([]byte, handshakeLen)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to read handshake: %w", err)
	}

	peerHs := newHandshake(pm.peer.InfoHash, PeerID{})
	err = peerHs.decode(buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to decode handshake: %w", err)
	}
	pm.peer.Conn = conn
	return nil
}

func (pm *PeerManager) acceptHandshake() error {
	_ = pm.peer.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer pm.peer.Conn.SetReadDeadline(time.Time{})
	buf := make([]byte, handshakeLen)
	_, err := io.ReadFull(pm.peer.Conn, buf)
	if err != nil {
		return fmt.Errorf("unable to read handshake: %w", err)
	}
	peerHs := newHandshake(pm.peer.InfoHash, PeerID{})
	err = peerHs.decode(buf)
	if err != nil {
		return fmt.Errorf("unable to decode handshake: %w", err)
	}
	pm.peer.InfoHash = peerHs.infoHash
	torrent := pm.storage.Get(pm.peer.InfoHash)
	if torrent == nil {
		return fmt.Errorf("torrent with info hash %s not found", pm.peer.InfoHash)
	}
	pm.torrent = torrent
	myHs := newHandshake(pm.peer.InfoHash, pm.clientId)
	_ = pm.peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer pm.peer.Conn.SetWriteDeadline(time.Time{})
	_, err = pm.peer.Conn.Write(myHs.encode())
	return err
}

func (pm *PeerManager) readMessages() {
	defer close(pm.incomeMessagesCh)
	_ = pm.peer.Conn.SetReadDeadline(time.Time{})
	for {
		bufLen := make([]byte, 4)
		_, err := io.ReadFull(pm.peer.Conn, bufLen)
		if err != nil {
			pm.log.Error().Err(err).Send()
			return
		}
		msgLen := binary.BigEndian.Uint32(bufLen)
		if msgLen == 0 { //keep-alive message
			continue
		}
		msgBuf := make([]byte, msgLen)
		_, err = io.ReadFull(pm.peer.Conn, msgBuf)
		if err != nil {
			pm.log.Error().Err(err).Send()
			return
		}
		pm.incomeMessagesCh <- &Message{
			ID:      messageId(msgBuf[0]),
			Payload: msgBuf[1:],
		}
	}
}

func (pm *PeerManager) sendMessage(message *Message) error {
	// allow to send msgUnChoke and msgBitfield even if amChoking
	if pm.amChoking.Load() && !(message.ID == msgBitfield || message.ID == msgUnChoke) {
		return fmt.Errorf("am choking")
	}
	pm.log.Debug().Int("messageId", int(message.ID)).Int("payload_len", len(message.Payload)).Msg("mgs sent")
	pm.outcomeMessageCh <- message
	return nil
}

func (pm *PeerManager) writeMessages() {
	for message := range pm.outcomeMessageCh {
		_ = pm.peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := pm.peer.Conn.Write(message.Encode())
		if err != nil {
			pm.log.Error().Err(err).Send()
		}
	}
}

func (pm *PeerManager) download() {
	for {
		task, err := pm.dm.generateTask()
		if err != nil {
			return
		}
		_ = pm.sendMessage(NewRequest(uint32(task.pieceIndex), uint32(task.begin), uint32(task.len)))
	}
}

func (pm *PeerManager) handleMessages() {
	for message := range pm.incomeMessagesCh {
		if !pm.IsAlive() {
			return
		}
		pm.log.Debug().Int("messageId", int(message.ID)).Int("payload_len", len(message.Payload)).Msg("mgs received")
		switch message.ID {
		case msgChoke:
			pm.amChoking.Store(true)
		case msgUnChoke:
			pm.amChoking.Store(false)
		case msgInterested:
			pm.peerInterested.Store(true)
		case msgNotInterested:
			pm.peerInterested.Store(false)
		case msgHave:
		case msgBitfield:
			bf, err := bitfield.FromPayload(message.Payload, pm.torrent.PiecesCount())
			if err != nil {
				close(pm.exit) // TODO may panic
				return
			}
			pm.peerBitfield = bf
		case msgRequest:
			index := binary.BigEndian.Uint32(message.Payload[:4])
			begin := binary.BigEndian.Uint32(message.Payload[4:8])
			length := binary.BigEndian.Uint32(message.Payload[8:12])
			offset := int(index)*pm.torrent.PieceLength + int(begin)
			if offset+int(length) > pm.torrent.Length {
				length = uint32(pm.torrent.Length - offset)
			}
			buf := make([]byte, length)
			_, err := pm.file.ReadAt(buf, int64(offset))
			if err != nil {
				pm.log.Error().Uint32("index", index).Uint32("begin", begin).Uint32("length", length).Int("offset", offset).Err(err).Send()
				close(pm.exit) // TODO may panic
				return
			}
			_ = pm.sendMessage(NewPiece(index, begin, buf))
		case msgPiece:
			index := binary.BigEndian.Uint32(message.Payload[:4])
			begin := binary.BigEndian.Uint32(message.Payload[4:8])
			data := message.Payload[8:]
			offset := int(index)*pm.torrent.PieceLength + int(begin)
			_, err := pm.file.WriteAt(data, int64(offset))
			if err != nil {
				pm.log.Error().Err(err).Send()
				close(pm.exit) // TODO may panic
				return
			}
			pm.dm.completeTask(downloadTask{
				pieceIndex: int(index),
				begin:      int(begin),
				len:        len(data),
			})
			pm.log.Info().Uint32("piece", index).Msg("block downloaded")
		case msgCancel:
		case msgPort:
		default:
			// ignore
		}
	}
}

func (pm *PeerManager) setLoggerFromCtx(ctx context.Context) {
	pm.log = log.Ctx(ctx).With().Str("peer_id", string(pm.clientId[:])).Str("remote_peer", pm.peer.Address()).Logger().
		Hook(zerolog.HookFunc(func(e *zerolog.Event, _ zerolog.Level, _ string) {
			infoHashStr := ""
			if pm.peer.InfoHash.IsZero() {
				infoHashStr = pm.peer.InfoHash.String()
			}
			e.Str("info_hash", infoHashStr)
		}))
}

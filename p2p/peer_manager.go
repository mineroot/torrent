package p2p

import (
	"context"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	handshakeLen = 68
	pstr         = "BitTorrent protocol"
)

type PeerManagers []*PeerManager

func (pms PeerManagers) FindAllBy(infoHash *Hash, ip net.IP) PeerManagers {
	foundPms := make(PeerManagers, 0, 10)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash && pm.peer.IP.Equal(ip) {
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
	conn             net.Conn
	isAlive          atomic.Bool
	incomeMessagesCh chan *Message
	outcomeMessageCh chan *Message
	bitfield         []byte
	amChoking        atomic.Bool
	amInterested     atomic.Bool
	peerChoking      atomic.Bool
	peerInterested   atomic.Bool
	log              zerolog.Logger
}

func NewPeerManager(clientId PeerID, storage StorageReader, peer Peer, conn net.Conn) *PeerManager {
	pm := &PeerManager{
		clientId:         clientId,
		storage:          storage,
		peer:             peer,
		conn:             conn,
		incomeMessagesCh: make(chan *Message, 512),
		outcomeMessageCh: make(chan *Message, 512),
	}
	pm.isAlive.Store(true)
	pm.amChoking.Store(true)
	pm.peerChoking.Store(true)
	return pm
}

func (pm *PeerManager) Run(ctx context.Context, wg *sync.WaitGroup, handshakeOkCh chan<- bool) {
	defer wg.Done()
	defer func() {
		close(pm.outcomeMessageCh)
		if pm.conn != nil {
			pm.conn.Close()
		}
		pm.isAlive.Store(false)
	}()
	if !pm.isAlive.Load() {
		panic("peer manager is not alive")
	}
	pm.setLoggerFromCtx(ctx)

	var err error
	if pm.conn == nil {
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
	if handshakeOkCh != nil {
		handshakeOkCh <- err == nil
		close(handshakeOkCh)
	}
	if err != nil {
		pm.log.Error().Err(err).Send()
		return
	}
	pm.log.Info().Msg("successful handshake")

	bitfield := pm.storage.GetBitfield(*pm.peer.InfoHash)
	pm.outcomeMessageCh <- NewBitfield(bitfield)
	go func() { // todo: for test
		if bitfield.IsCompleted() {
			pm.log.Info().Msg("seeding")
			pm.outcomeMessageCh <- NewUnChoke()
		} else {
			pm.log.Info().Msg("downloading")
			for {
				if !pm.amChoking.Load() {
					pm.outcomeMessageCh <- NewRequest()
					break
				}
				time.Sleep(time.Millisecond * 100)
			}
		}
	}()

	go pm.readMessages()
	go pm.writeMessages()
	go pm.handleMessages()

	<-ctx.Done()
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
	pm.conn = conn
	return nil
}

func (pm *PeerManager) acceptHandshake() error {
	_ = pm.conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	defer pm.conn.SetReadDeadline(time.Time{})
	buf := make([]byte, handshakeLen)
	_, err := io.ReadFull(pm.conn, buf)
	if err != nil {
		return fmt.Errorf("unable to read handshake: %w", err)
	}
	peerHs := newHandshake(pm.peer.InfoHash, PeerID{})
	err = peerHs.decode(buf)
	if err != nil {
		return fmt.Errorf("unable to decode handshake: %w", err)
	}
	pm.peer.InfoHash = peerHs.infoHash
	torrent := pm.storage.Get(*pm.peer.InfoHash)
	if torrent == nil {
		return fmt.Errorf("torrent with info hash %s not found", pm.peer.InfoHash)
	}
	myHs := newHandshake(pm.peer.InfoHash, pm.clientId)
	_ = pm.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	defer pm.conn.SetWriteDeadline(time.Time{})
	_, err = pm.conn.Write(myHs.encode())
	return err
}

func (pm *PeerManager) readMessages() {
	defer close(pm.incomeMessagesCh)
	_ = pm.conn.SetReadDeadline(time.Time{})
	for {
		bufLen := make([]byte, 4)
		_, err := io.ReadFull(pm.conn, bufLen)
		if err != nil {
			pm.log.Error().Err(err).Send()
			return
		}
		msgLen := binary.BigEndian.Uint32(bufLen)
		if msgLen == 0 { //keep-alive message
			continue
		}
		msgBuf := make([]byte, msgLen)
		_, err = io.ReadFull(pm.conn, msgBuf)
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

func (pm *PeerManager) writeMessages() {
	for message := range pm.outcomeMessageCh {
		_ = pm.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		_, err := pm.conn.Write(message.Encode())
		if err != nil {
			pm.log.Error().Err(err).Send()
		}
	}
}

func (pm *PeerManager) handleMessages() {
	for message := range pm.incomeMessagesCh {
		pm.log.Info().Int("messageId", int(message.ID)).Str("payload", hex.EncodeToString(message.Payload)).Msg("new message")
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
		case msgRequest:
		case msgPiece:
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
			if pm.peer.InfoHash != nil {
				infoHashStr = pm.peer.InfoHash.String()
			}
			e.Str("info_hash", infoHashStr)
		}))
}

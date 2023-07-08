package p2p

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"log"
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
	storage          StorageGetter
	peer             Peer
	conn             net.Conn
	isAlive          atomic.Bool
	incomeMessagesCh chan *Message
	bitfield         []byte
	amChoking        bool
	amInterested     bool
	peerChoking      bool
	peerInterested   bool
}

func NewPeerManager(storage StorageGetter, peer Peer, conn net.Conn) *PeerManager {
	pm := &PeerManager{
		storage:          storage,
		peer:             peer,
		conn:             conn,
		incomeMessagesCh: make(chan *Message, 512),
		amChoking:        true,
		peerChoking:      true,
	}
	pm.isAlive.Store(true)
	return pm
}

func (pm *PeerManager) Run(ctx context.Context, wg *sync.WaitGroup) {
	if !pm.isAlive.Load() {
		panic("peer manager is not alive")
	}
	defer wg.Done()
	defer func() {
		if pm.conn != nil {
			pm.conn.Close()
		}
		pm.isAlive.Store(false)
	}()
	if pm.conn == nil {
		err := pm.sendHandshake(ctx)
		if err != nil {
			log.Printf("%s: unable to send handshake: %s", pm.peer.Address(), err)
			return
		}
	} else {
		err := pm.acceptHandshake()
		if err != nil {
			log.Printf("%s: unable to accept handshake: %s", pm.peer.Address(), err)
			return
		}
	}
	log.Printf("%s: successful handshake", pm.peer.Address())

	go pm.readMessages()
	go pm.handleMessages()
	<-ctx.Done()
}

func (pm *PeerManager) IsAlive() bool {
	return pm.isAlive.Load()
}

func (pm *PeerManager) sendHandshake(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	myHs := newHandshake(pm.peer.InfoHash, MyPeerID())
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
	myHs := newHandshake(pm.peer.InfoHash, MyPeerID())
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
			return
		}
		msgLen := binary.BigEndian.Uint32(bufLen)
		if msgLen == 0 { //keep-alive message
			continue
		}
		msgBuf := make([]byte, msgLen)
		_, err = io.ReadFull(pm.conn, msgBuf)
		if err != nil {
			return
		}
		pm.incomeMessagesCh <- &Message{
			ID:      messageId(msgBuf[0]),
			Payload: msgBuf[1:],
		}
	}
}

func (pm *PeerManager) handleMessages() {
	for message := range pm.incomeMessagesCh {
		switch message.ID {
		case msgChoke:
			pm.amChoking = true
		case msgUnChoke:
			pm.amChoking = false
		case msgInterested:
			pm.peerInterested = true
		case msgNotInterested:
			pm.peerInterested = false
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

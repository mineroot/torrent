package p2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

const (
	handshakeLen = 68
	pstr         = "BitTorrent protocol"
)

type PeerManagers map[ipv4]*PeerManager

type PeerManager struct {
	conn           net.Conn
	peer           Peer
	infoHash       Hash
	amChoking      bool
	amInterested   bool
	peerChoking    bool
	peerInterested bool
}

func NewPeerManager(peer Peer, infoHash Hash) *PeerManager {
	return &PeerManager{
		peer:        peer,
		infoHash:    infoHash,
		amChoking:   true,
		peerChoking: true,
	}
}

func (pm *PeerManager) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	defer func() {
		if pm.conn != nil {
			pm.conn.Close()
			pm.conn = nil
		}
	}()
	err := pm.handshake(ctx)
	if err != nil {
		log.Printf("%s: unable to handshake: %s", pm.peer.Address(), err)
		return
	}
	log.Printf("%s: successful handshake", pm.peer.Address())
	<-ctx.Done()

}

func (pm *PeerManager) IsAlive() bool {
	return pm.conn != nil
}

func (pm *PeerManager) Close() error {
	if pm.conn == nil {
		return nil
	}
	return pm.conn.Close()
}

func (pm *PeerManager) handshake(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	myHs := newHandshake(pm.infoHash, MyPeerID())
	dialer := &net.Dialer{
		KeepAlive: 30 * time.Second,
	}
	conn, err := dialer.DialContext(ctx, "tcp", pm.peer.Address())
	if err != nil {
		return fmt.Errorf("unable to establish conn: %w", err)
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	_, err = conn.Write(myHs.encode())
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to write handshake: %w", err)
	}
	buf := make([]byte, handshakeLen)
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to read handshake: %w", err)
	}

	peerHs := newHandshake(pm.infoHash, PeerID{})
	err = peerHs.decode(buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to decode handshake: %w", err)
	}
	pm.conn = conn
	return nil
}

type handshake struct {
	infoHash Hash
	peerID   PeerID
}

func newHandshake(infoHash Hash, peerId PeerID) *handshake {
	return &handshake{
		infoHash: infoHash,
		peerID:   peerId,
	}
}

// encode <pstrlen><pstr><reserved><info_hash><peer_id>
func (h *handshake) encode() []byte {
	buf := make([]byte, handshakeLen)
	buf[0] = byte(len(pstr))
	curr := 1
	curr += copy(buf[curr:], pstr)
	curr += copy(buf[curr:], make([]byte, 8)) // 8 reserved bytes
	curr += copy(buf[curr:], h.infoHash[:])
	curr += copy(buf[curr:], h.peerID[:])
	return buf
}

func (h *handshake) decode(raw []byte) error {
	if h == nil {
		panic("h must be not nil")
	}
	r := bytes.NewReader(raw)
	// read pstrlen
	if pstrLen, err := r.ReadByte(); err != nil || pstrLen != byte(len(pstr)) {
		return fmt.Errorf("invalid handshake: unable to read pstrlen")
	}
	// read pstr
	buf := make([]byte, len(pstr))
	if _, err := io.ReadFull(r, buf); err != nil || string(buf) != pstr {
		return fmt.Errorf("invalid handshake: unable to read pstr")
	}
	// read reserved 8 bytes
	buf = make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("invalid handshake: unable to read reserved bytes")
	}
	// read info_hash
	buf = make([]byte, HashSize)
	if _, err := io.ReadFull(r, buf); err != nil || !bytes.Equal(buf, h.infoHash[:]) {
		return fmt.Errorf("invalid handshake: unable to read info_hash")
	}
	// read peer_id
	buf = make([]byte, PeerIdSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("invalid handshake: unable to read peer_id")
	}
	h.peerID = PeerID(buf)

	return nil
}

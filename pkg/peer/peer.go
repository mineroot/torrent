package peer

import (
	"fmt"
	"io"
	"net"
	"time"

	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
)

const IdSize = 20

type ID [IdSize]byte

func (id ID) PeerId() [IdSize]byte {
	return id
}

type Peers []Peer

type Peer struct {
	InfoHash torrent.Hash
	IP       net.IP
	Port     uint16
	Conn     net.Conn
	//torrent *torrent.File // TODO maybe?
}

func (p *Peer) Address() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

func (p *Peer) sendHandshake(id ID) error {
	conn, err := net.DialTimeout("tcp", p.Address(), 5*time.Second)
	if err != nil {
		return fmt.Errorf("unable to establish conn: %w", err)
	}
	defer conn.SetDeadline(time.Time{})
	myHs := newHandshake(p.InfoHash, id)
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

	peerHs := newHandshake(p.InfoHash, ID{})
	err = peerHs.decode(buf)
	if err != nil {
		conn.Close()
		return fmt.Errorf("unable to decode handshake: %w", err)
	}
	p.Conn = conn
	return nil
}

func (p *Peer) acceptHandshake(id ID, storage storage.Reader) (*torrent.File, error) {
	defer p.Conn.SetDeadline(time.Time{})
	buf := make([]byte, handshakeLen)
	_ = p.Conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	_, err := io.ReadFull(p.Conn, buf)
	if err != nil {
		return nil, fmt.Errorf("unable to read handshake: %w", err)
	}
	peerHs := newHandshake(p.InfoHash, ID{})
	err = peerHs.decode(buf)
	if err != nil {
		return nil, fmt.Errorf("unable to decode handshake: %w", err)
	}
	p.InfoHash = peerHs.infoHash
	t := storage.Get(p.InfoHash)
	if t == nil {
		return nil, fmt.Errorf("torrent with info hash %s not found", p.InfoHash)
	}
	myHs := newHandshake(p.InfoHash, id)
	_ = p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
	_, err = p.Conn.Write(myHs.encode())
	if err != nil {
		return nil, fmt.Errorf("unable to write handshake: %w", err)
	}
	return t, err
}

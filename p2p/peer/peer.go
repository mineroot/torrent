package peer

import (
	"fmt"
	"net"
	"torrent/p2p/torrent"
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
}

func (p Peer) Address() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

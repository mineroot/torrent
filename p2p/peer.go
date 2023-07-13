package p2p

import (
	"fmt"
	"net"
	"torrent/p2p/torrent"
)

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

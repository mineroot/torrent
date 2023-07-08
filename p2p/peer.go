package p2p

import (
	"fmt"
	"net"
)

type Peers []Peer

type Peer struct {
	InfoHash *Hash
	IP       net.IP
	Port     uint16
	Conn     net.Conn
}

func (p Peer) Address() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

package p2p

import "net"

type ipv4 [net.IPv4len]byte

func ipv4FromNetIP(ip net.IP) ipv4 {
	if len(ip) < net.IPv4len {
		panic("ip is less than 4 bytes")
	}
	return (ipv4)(ip)
}

type Peer struct {
	IP   net.IP
	Port uint16
}

type Peers map[ipv4]Peer

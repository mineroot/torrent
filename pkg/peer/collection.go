package peer

import (
	"net"

	"github.com/mineroot/torrent/pkg/torrent"
)

type Managers []*Manager

func (pms Managers) FindAliveByInfoHashAndIp(infoHash torrent.Hash, ip net.IP) Managers {
	foundPms := make(Managers, 0)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash && pm.peer.IP.Equal(ip) && pm.IsAlive() {
			foundPms = append(foundPms, pm)
		}
	}
	return foundPms
}

func (pms Managers) FindAlive() Managers {
	alivePms := make([]*Manager, 0, len(pms))
	for _, pm := range pms {
		if pm.IsAlive() {
			alivePms = append(alivePms, pm)
		}
	}
	return alivePms
}

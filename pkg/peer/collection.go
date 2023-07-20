package peer

import (
	"net"

	"github.com/mineroot/torrent/pkg/torrent"
)

type Managers []*Manager

func (pms Managers) FindByInfoHashAndIp(infoHash torrent.Hash, ip net.IP) Managers {
	foundPms := make(Managers, 0, 10)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash && pm.peer.IP.Equal(ip) {
			foundPms = append(foundPms, pm)
		}
	}
	return foundPms
}

func (pms Managers) FindByInfoHash(infoHash torrent.Hash) Managers {
	foundPms := make(Managers, 0, 100)
	for _, pm := range pms {
		if pm.peer.InfoHash == infoHash {
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

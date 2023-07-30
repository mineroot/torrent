package peer

import (
	"github.com/stretchr/testify/assert"
	"net"
	"testing"

	"github.com/mineroot/torrent/pkg/torrent"
)

func TestManagers(t *testing.T) {
	hash1Ip1Alive := NewManager(ID{}, nil, nil, Peer{InfoHash: torrent.Hash{0x1}, IP: net.IP{1}}, nil, nil, nil)
	anotherHash1Ip1Alive := NewManager(ID{}, nil, nil, Peer{InfoHash: torrent.Hash{0x1}, IP: net.IP{1}}, nil, nil, nil)
	hash1Ip2Alive := NewManager(ID{}, nil, nil, Peer{InfoHash: torrent.Hash{0x1}, IP: net.IP{2}}, nil, nil, nil)
	hash1Ip2Dead := NewManager(ID{}, nil, nil, Peer{InfoHash: torrent.Hash{0x1}, IP: net.IP{2}}, nil, nil, nil)
	hash1Ip2Dead.isAlive.Store(false)

	pms := Managers{hash1Ip1Alive, anotherHash1Ip1Alive, hash1Ip2Alive, hash1Ip2Dead}
	assert.ElementsMatch(t, pms.FindAlive(), Managers{hash1Ip1Alive, anotherHash1Ip1Alive, hash1Ip2Alive})
	assert.ElementsMatch(t, pms.FindAliveByInfoHashAndIp(torrent.Hash{0x1}, net.IP{1}), Managers{hash1Ip1Alive, anotherHash1Ip1Alive})
}

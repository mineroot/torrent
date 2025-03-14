package tracker

import (
	"encoding/binary"
	"fmt"
	bencode2 "github.com/mineroot/torrent/pkg/bencode"
	"net"
	"time"

	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/torrent"
)

type response struct {
	infoHash    torrent.Hash
	failure     string
	warning     string
	interval    time.Duration
	minInterval time.Duration
	trackerId   string
	peers       peer.Peers
}

func newTrackerResponse(infoHash torrent.Hash) *response {
	return &response{
		infoHash: infoHash,
	}
}

func (r *response) unmarshal(benType bencode2.BenType) error {
	if r == nil {
		panic("response must be not nil")
	}
	dict, ok := benType.(*bencode2.Dictionary)
	if !ok {
		return fmt.Errorf("response must be a dictionary")
	}
	failure, ok := dict.Get("failure").(*bencode2.String)
	if ok {
		r.failure = failure.Value()
		return fmt.Errorf("failure: %s", r.failure)
	}
	warning, ok := dict.Get("warning").(*bencode2.String)
	if ok {
		r.warning = warning.Value()
	}
	interval, ok := dict.Get("interval").(*bencode2.Integer)
	if ok {
		r.interval = time.Second * time.Duration(interval.Value())
	} else {
		r.interval = 900 * time.Second
	}
	minInterval, ok := dict.Get("minInterval").(*bencode2.Integer)
	if ok {
		r.minInterval = time.Second * time.Duration(minInterval.Value())
	}

	peers, ok := dict.Get("peers").(*bencode2.String)
	if !ok {
		return fmt.Errorf("peers must be bytes")
	}
	const peerSize = 6
	peersBuf := []byte(peers.Value())
	if len(peersBuf)%peerSize != 0 {
		return fmt.Errorf("malformed peers bytes")
	}
	peersCount := len(peersBuf) / peerSize
	r.peers = make(peer.Peers, peersCount)
	for i := 0; i < peersCount; i++ {
		offset := i * peerSize
		p := peersBuf[offset : offset+peerSize]
		r.peers[i] = peer.Peer{
			InfoHash: r.infoHash,
			IP:       p[:net.IPv4len],
			Port:     binary.BigEndian.Uint16(p[net.IPv4len:]),
		}
	}
	return nil
}

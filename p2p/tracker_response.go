package p2p

import (
	"encoding/binary"
	"fmt"
	"net"
	"time"
	"torrent/bencode"
)

type trackerResponse struct {
	infoHash    Hash
	failure     string
	warning     string
	interval    time.Duration
	minInterval time.Duration
	trackerId   string
	peers       Peers
}

func newTrackerResponse(infoHash Hash) *trackerResponse {
	return &trackerResponse{
		infoHash: infoHash,
	}
}

func (r *trackerResponse) unmarshal(benType bencode.BenType) error {
	if r == nil {
		panic("trackerResponse must be not nil")
	}
	dict, ok := benType.(*bencode.Dictionary)
	if !ok {
		return fmt.Errorf("response must be a dictionary")
	}
	failure, ok := dict.Get("failure").(*bencode.String)
	if ok {
		r.failure = failure.Value()
		return fmt.Errorf("failure: %s", r.failure)
	}
	warning, ok := dict.Get("warning").(*bencode.String)
	if ok {
		r.warning = warning.Value()
	}
	interval, ok := dict.Get("interval").(*bencode.Integer)
	if ok {
		r.interval = time.Second * time.Duration(interval.Value())
	} else {
		r.interval = 900 * time.Second
	}
	minInterval, ok := dict.Get("minInterval").(*bencode.Integer)
	if ok {
		r.minInterval = time.Second * time.Duration(minInterval.Value())
	}

	peers, ok := dict.Get("peers").(*bencode.String)
	if !ok {
		return fmt.Errorf("peers must be bytes")
	}
	const peerSize = 6
	peersBuf := []byte(peers.Value())
	if len(peersBuf)%peerSize != 0 {
		return fmt.Errorf("malformed peers bytes")
	}
	peersCount := len(peersBuf) / peerSize
	r.peers = make(Peers, peersCount)
	for i := 0; i < peersCount; i++ {
		offset := i * peerSize
		peer := peersBuf[offset : offset+peerSize]
		r.peers[i] = Peer{
			InfoHash: r.infoHash,
			IP:       peer[:net.IPv4len],
			Port:     binary.BigEndian.Uint16(peer[net.IPv4len:]),
		}
	}

	return nil
}

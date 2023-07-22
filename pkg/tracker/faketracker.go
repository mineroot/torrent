package tracker

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync"

	"github.com/mineroot/torrent/pkg/bencode"
	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/torrent"
)

type FakeTracker struct {
	*http.Server
	lock            sync.RWMutex
	peersByInfoHash map[torrent.Hash]map[string]peer.Peer
}

func NewFakeTracker(addr string) *FakeTracker {
	ft := &FakeTracker{peersByInfoHash: make(map[torrent.Hash]map[string]peer.Peer)}
	mux := http.NewServeMux()
	mux.HandleFunc("/announce", ft.announceHandler)
	ft.Server = &http.Server{Addr: addr, Handler: mux}
	return ft
}

func (ft *FakeTracker) announceHandler(w http.ResponseWriter, r *http.Request) {
	// parse info_hash
	infoHashBytes := []byte(r.URL.Query().Get("info_hash"))
	if len(infoHashBytes) != torrent.HashSize {
		w.Header().Set("Content-Type", "text/plain")
		if err := ft.writeErrorResponse(w, "invalid info hash"); err != nil {
			log.Println(err)
		}
		return
	}
	infoHash := torrent.Hash(infoHashBytes)

	// parse port
	port, err := strconv.ParseUint(r.URL.Query().Get("port"), 10, 16)
	if err != nil {
		log.Println(err)
		w.Header().Set("Content-Type", "text/plain")
		if err := ft.writeErrorResponse(w, "invalid port"); err != nil {
			log.Println(err)
		}
		return
	}

	// parse ip
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Println(err)
		w.Header().Set("Content-Type", "text/plain")
		if err := ft.writeErrorResponse(w, "invalid id"); err != nil {
			log.Println(err)
		}
		return
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Println(err)
		w.Header().Set("Content-Type", "text/plain")
		if err := ft.writeErrorResponse(w, "invalid id"); err != nil {
			log.Println(err)
		}
		return
	}
	p := peer.Peer{
		InfoHash: infoHash,
		IP:       ip,
		Port:     uint16(port),
	}

	if r.URL.Query().Get("event") == stopped {
		log.Println("delete")
		ft.deletePeer(p)
	} else {
		log.Println("add")
		ft.addPeer(p)
	}
	peersStr := ft.encodePeers(infoHash)
	if err := ft.writeSuccessResponse(w, peersStr); err != nil {
		log.Println(err)
	}
	ft.lock.RLock()
	log.Printf("%+v\n", ft.peersByInfoHash)
	ft.lock.RUnlock()
	log.Println()
}

func (ft *FakeTracker) writeSuccessResponse(w io.Writer, peersStr string) error {
	dict := bencode.NewDictionary(map[bencode.String]bencode.BenType{
		*bencode.NewString("interval"):   bencode.NewInteger(30),
		*bencode.NewString("complete"):   bencode.NewInteger(0),
		*bencode.NewString("incomplete"): bencode.NewInteger(0),
		*bencode.NewString("peers"):      bencode.NewString(peersStr),
	})
	return dict.Encode(w)
}

func (ft *FakeTracker) writeErrorResponse(w io.Writer, failureReason string) error {
	dict := bencode.NewDictionary(map[bencode.String]bencode.BenType{
		*bencode.NewString("failure reason"): bencode.NewString(failureReason),
	})
	return dict.Encode(w)
}

func (ft *FakeTracker) encodePeers(infoHash torrent.Hash) string {
	const bytesPerPeer = 6
	ft.lock.RLock()
	defer ft.lock.RUnlock()
	peers := ft.peersByInfoHash[infoHash]
	peersBytes := make([]byte, 0, len(peers)*bytesPerPeer)
	for _, p := range peers {
		ipBytes := make([]byte, 4)
		binary.BigEndian.PutUint32(ipBytes, binary.BigEndian.Uint32(p.IP))
		portBytes := make([]byte, 2)
		binary.BigEndian.PutUint16(portBytes, p.Port)
		peersBytes = append(peersBytes, ipBytes...)
		peersBytes = append(peersBytes, portBytes...)
	}
	return string(peersBytes)
}

func (ft *FakeTracker) addPeer(p peer.Peer) {
	ft.lock.Lock()
	defer ft.lock.Unlock()
	peersByAddress, ok := ft.peersByInfoHash[p.InfoHash]
	if !ok {
		peersByAddress = make(map[string]peer.Peer)
	}
	peersByAddress[p.Address()] = p
	ft.peersByInfoHash[p.InfoHash] = peersByAddress
}

func (ft *FakeTracker) deletePeer(p peer.Peer) {
	ft.lock.Lock()
	defer ft.lock.Unlock()
	if peersByAddress, ok := ft.peersByInfoHash[p.InfoHash]; ok {
		delete(peersByAddress, p.Address())
	}
}

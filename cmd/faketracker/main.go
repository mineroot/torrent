package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"github.com/mineroot/torrent/pkg/bencode"
	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/torrent"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
)

var (
	mu              sync.RWMutex
	peersByInfoHash = make(map[torrent.Hash]peer.Peers)
)

func main() {
	server := &http.Server{Addr: ":8080"}
	http.HandleFunc("/announce", announceHandler)

	go func() {
		fmt.Println(server.ListenAndServe())
	}()

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit
	_ = server.Shutdown(context.TODO())
}

func announceHandler(w http.ResponseWriter, r *http.Request) {
	// parse info_hash
	infoHashBytes := []byte(r.URL.Query().Get("info_hash"))
	if len(infoHashBytes) != torrent.HashSize {
		w.Header().Set("Content-Type", "text/plain")
		if err := writeErrorResponse(w, "invalid info hash"); err != nil {
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
		if err := writeErrorResponse(w, "invalid port"); err != nil {
			log.Println(err)
		}
		return
	}

	// parse ip
	ipStr, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		log.Println(err)
		w.Header().Set("Content-Type", "text/plain")
		if err := writeErrorResponse(w, "invalid id"); err != nil {
			log.Println(err)
		}
		return
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		log.Println(err)
		w.Header().Set("Content-Type", "text/plain")
		if err := writeErrorResponse(w, "invalid id"); err != nil {
			log.Println(err)
		}
		return
	}
	p := peer.Peer{
		InfoHash: infoHash,
		IP:       ip,
		Port:     uint16(port),
	}
	mu.Lock()
	peers := peersByInfoHash[infoHash]
	peersByInfoHash[infoHash] = append(peers, p) // todo check for uniqueness
	//fmt.Printf("%+v\n", peersByInfoHash)
	mu.Unlock()
	peersStr := encodePeers(infoHash)
	if err := writeSuccessResponse(w, peersStr); err != nil {
		log.Println(err)
	}
	log.Println("success")
}

func writeSuccessResponse(w io.Writer, peersStr string) error {
	dict := bencode.NewDictionary(map[bencode.String]bencode.BenType{
		*bencode.NewString("interval"):   bencode.NewInteger(30),
		*bencode.NewString("complete"):   bencode.NewInteger(0),
		*bencode.NewString("incomplete"): bencode.NewInteger(0),
		*bencode.NewString("peers"):      bencode.NewString(peersStr),
	})
	return dict.Encode(w)
}

func writeErrorResponse(w io.Writer, failureReason string) error {
	dict := bencode.NewDictionary(map[bencode.String]bencode.BenType{
		*bencode.NewString("failure reason"): bencode.NewString(failureReason),
	})
	return dict.Encode(w)
}

func encodePeers(infoHash torrent.Hash) string {
	const bytesPerPeer = 6
	mu.RLock()
	defer mu.RUnlock()
	peers := peersByInfoHash[infoHash]
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

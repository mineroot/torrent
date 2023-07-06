package p2p

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
	"torrent/bencode"
)

type Client struct {
	httpClient *http.Client
	torrent    *TorrentFile
	lock       sync.RWMutex
	peers      Peers
}

func NewClient(torrent *TorrentFile) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		torrent:    torrent,
		peers:      make(Peers),
	}
}

func (c *Client) PeerID() Hash {
	return (Hash)([]byte("-GO0001-random_bytes"))
}

func (c *Client) Run(ctx context.Context) error {
	err := c.trackerRequest(ctx, started)
	if err != nil {
		return err
	}
	// TODO: start goroutine for sending regular requests
	<-ctx.Done()
	err = c.trackerStoppedRequest(ctx)
	if err != nil {
		return err
	}
	return ctx.Err()
}

type event string

const (
	started   event = "started"
	stopped         = "stopped"
	completed       = "completed"
)

func (c *Client) trackerRequest(ctx context.Context, event event) error {
	url, err := c.torrent.buildTrackerURL(c.PeerID(), 6881, event)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(b)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		return err
	}
	response := &trackerResponse{}
	err = response.unmarshal(decoded)
	if err != nil {
		return err
	}
	fmt.Printf("%+v\n", response)

	return nil
}

func (c *Client) trackerStoppedRequest(ctx context.Context) error {
	url, err := c.torrent.buildTrackerURL(c.PeerID(), 6881, stopped)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}
	_, err = c.httpClient.Do(req)
	return err
}

type trackerResponse struct {
	failure     string
	warning     string
	interval    time.Duration
	minInterval time.Duration
	trackerId   string
	peers       Peers
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
		ip := peer[:net.IPv4len]
		r.peers[ipv4FromNetIP(ip)] = Peer{
			IP:   ip,
			Port: binary.BigEndian.Uint16(peer[net.IPv4len:]),
		}
	}

	return nil
}

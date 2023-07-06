package p2p

import (
	"bytes"
	"context"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
	"torrent/bencode"
)

const PeerIdSize = 20

type PeerID [PeerIdSize]byte

func MyPeerID() PeerID {
	return PeerID([]byte("-GO0001-random_bytes"))
}

type Client struct {
	httpClient *http.Client
	torrent    *TorrentFile
	peersCh    chan Peers
	pmsLock    sync.RWMutex
	pms        PeerManagers
	interval   time.Duration
}

func NewClient(torrent *TorrentFile) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		torrent:    torrent,
		peersCh:    make(chan Peers, 1),
		pms:        make(PeerManagers),
	}
}

func (c *Client) Run(ctx context.Context) error {
	err := c.trackerRequest(ctx, started)
	if err != nil {
		log.Printf("failed to handle tracker request %s", err)
	}
	var wg sync.WaitGroup
	wg.Add(2)
	go c.trackerRegularRequests(ctx, &wg)
	go c.managePeers(ctx, &wg)
	wg.Wait()
	err = c.trackerStoppedRequest(ctx)
	if err != nil {
		return err
	}
	c.stop()
	return ctx.Err()
}

func (c *Client) stop() {
	c.pmsLock.RLock()
	defer c.pmsLock.RUnlock()
	for _, pm := range c.pms {
		_ = pm.Close()
	}
}

type event string

const (
	started   event = "started"
	regular         = ""
	completed       = "completed"
	stopped         = "stopped"
)

func (c *Client) trackerRegularRequests(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			err := c.trackerRequest(ctx, regular)
			if err != nil {
				log.Printf("failed to handle tracker request %s", err)
			}
		case <-ctx.Done():
			close(c.peersCh)
			return
		}
	}
}

func (c *Client) managePeers(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case peers, ok := <-c.peersCh:
			if !ok {
				return
			}
			c.pmsLock.Lock()
			for ip, peer := range peers {
				_, ok = c.pms[ip]
				if !ok {
					pm := NewPeerManager(peer, c.torrent.InfoHash)
					c.pms[ip] = pm
					wg.Add(1)
					go pm.Run(ctx, wg)
				}
			}
			c.pmsLock.Unlock()
		case <-ticker.C:
			c.pmsLock.Lock()
			before := len(c.pms)
			for ip, pm := range c.pms {
				if !pm.IsAlive() {
					delete(c.pms, ip)
				}
			}
			log.Printf("deleted %d peers, total peers now %d", before-len(c.pms), len(c.pms))
			c.pmsLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) trackerRequest(ctx context.Context, event event) error {
	url, err := c.torrent.buildTrackerURL(MyPeerID(), 6881, event)
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
	c.interval = response.interval
	c.peersCh <- response.peers

	return nil
}

func (c *Client) trackerStoppedRequest(ctx context.Context) error {
	url, err := c.torrent.buildTrackerURL(MyPeerID(), 6881, stopped)
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

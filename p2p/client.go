package p2p

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
	"torrent/bencode"
)

const PeerIdSize = 20
const listenPort = 6881

type PeerID [PeerIdSize]byte

func MyPeerID() PeerID {
	return PeerID([]byte("-GO0001-random_bytes"))
}

type Client struct {
	httpClient    *http.Client
	storage       *Storage
	peersCh       chan Peers
	pmsLock       sync.RWMutex
	pms           PeerManagers
	intervalsLock sync.RWMutex
	intervals     map[Hash]time.Duration
}

func NewClient(storage *Storage) *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 15 * time.Second},
		storage:    storage,
		peersCh:    make(chan Peers, 1),
		pms:        make(PeerManagers, 0, 512),
		intervals:  make(map[Hash]time.Duration),
	}
}

func (c *Client) Run(ctx context.Context) error {
	c.trackerRequests(ctx, started)
	var wg sync.WaitGroup
	wg.Add(3)
	go c.listen(ctx, &wg)
	go c.managePeers(ctx, &wg)
	go c.trackerRegularRequests(ctx, &wg)
	wg.Wait()
	close(c.peersCh)
	c.trackerRequests(ctx, stopped)
	return ctx.Err()
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
	for torrent := range c.storage.IterateTorrents() {
		torrent := torrent
		go func() {
			c.intervalsLock.RLock()
			ticker := time.NewTicker(c.intervals[torrent.InfoHash])
			c.intervalsLock.RUnlock()
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					c.trackerRequest(ctx, torrent, regular, wg)
				case <-ctx.Done():
					return
				}
			}
		}()
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
			for _, peer := range peers {
				foundPms := c.pms.FindAllBy(peer.InfoHash, peer.IP)
				// peer.InfoHash == nil means peer has just connected to us
				// len(foundPms) == 0 means there are no pm with such infoHash and IP
				// always add peers who connected to us OR new peers sent from tracker
				if peer.InfoHash == nil || len(foundPms) == 0 {
					pm := NewPeerManager(c.storage, peer, peer.Conn)
					c.pms = append(c.pms, pm)
					wg.Add(1)
					go pm.Run(ctx, wg)
				}
			}
			c.pmsLock.Unlock()
		case <-ticker.C:
			c.pmsLock.Lock()
			before := len(c.pms)
			c.pms = c.pms.FindAlive()
			log.Printf("deleted %d peers, total peers now %d", before-len(c.pms), len(c.pms))
			c.pmsLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) listen(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	var lc net.ListenConfig

	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", listenPort))

	if err != nil {
		log.Printf("unable to listen port %d", listenPort)
		return
	}

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				str := err.Error()
				if strings.Contains(str, "use of closed network connection") {
					return
				}
				continue
			}
			if conn.RemoteAddr().Network() != "tcp" {
				conn.Close()
				continue
			}
			log.Printf("incoming connection from %s", conn.RemoteAddr().String())
			addr := conn.RemoteAddr().(*net.TCPAddr)
			peer := Peer{
				InfoHash: nil,
				IP:       addr.IP,
				Port:     uint16(addr.Port),
				Conn:     conn,
			}
			c.peersCh <- Peers{peer}
		}
	}()
	<-ctx.Done()
	listener.Close()
}

func (c *Client) trackerRequests(ctx context.Context, event event) {
	var wg sync.WaitGroup
	wg.Add(c.storage.Len())
	for torrent := range c.storage.IterateTorrents() {
		go c.trackerRequest(ctx, torrent, event, &wg)
	}
	wg.Wait()
}

func (c *Client) trackerRequest(ctx context.Context, torrent *TorrentFile, event event, wg *sync.WaitGroup) {
	defer wg.Done()
	url, err := torrent.buildTrackerURL(MyPeerID(), listenPort, event)
	if err != nil {
		log.Println(err)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Println(err)
		return
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Println(err)
		return
	}
	defer resp.Body.Close()
	if event == stopped {
		return // no need to decode response
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return
	}

	buf := bytes.NewBuffer(b)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		log.Println(err)
		return
	}
	response := newTrackerResponse(torrent.InfoHash)
	err = response.unmarshal(decoded)

	if err != nil {
		log.Println(err)
		return
	}
	c.intervalsLock.Lock()
	c.intervals[torrent.InfoHash] = response.interval
	defer c.intervalsLock.Unlock()
	c.peersCh <- response.peers
}

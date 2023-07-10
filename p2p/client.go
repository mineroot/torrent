package p2p

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
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

type Client struct {
	id            PeerID
	httpClient    *http.Client
	storage       *Storage
	peersCh       chan Peers
	pmsLock       sync.RWMutex
	pms           PeerManagers
	intervalsLock sync.RWMutex
	intervals     map[Hash]time.Duration
}

func NewClient(id PeerID, storage *Storage) *Client {
	return &Client{
		id:         id,
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
	go c.listen(ctx, &wg, nil)
	go c.managePeers(ctx, &wg)
	go c.trackerRegularRequests(ctx, &wg)
	wg.Wait()
	close(c.peersCh)
	// pass new ctx, because old one is already canceled
	c.trackerRequests(log.Ctx(ctx).WithContext(context.Background()), stopped)
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
	for torrent := range c.storage.Iterator() {
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
	l := log.Ctx(ctx)
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
					pm := NewPeerManager(c.id, c.storage, peer, peer.Conn)
					c.pms = append(c.pms, pm)
					wg.Add(1)
					go pm.Run(ctx, wg, nil)
				}
			}
			c.pmsLock.Unlock()
		case <-ticker.C:
			c.pmsLock.Lock()
			before := len(c.pms)
			c.pms = c.pms.FindAlive()
			l.Info().
				Int("dead", before-len(c.pms)).
				Int("alive", len(c.pms)).
				Msg("dead peers cleanup")
			c.pmsLock.Unlock()
		case <-ctx.Done():
			return
		}
	}
}

func (c *Client) listen(ctx context.Context, wg *sync.WaitGroup, started chan<- struct{}) {
	defer wg.Done()
	var lc net.ListenConfig
	l := log.Ctx(ctx)

	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", listenPort))
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to listen port %d: %w", listenPort, err)).
			Send()
		return
	}

	go func() {
		if started != nil {
			close(started)
		}
		for {
			conn, err := listener.Accept()
			if err != nil {
				str := err.Error()
				// TODO ?
				if strings.Contains(str, "use of closed network connection") {
					return
				}
				continue
			}
			if conn.RemoteAddr().Network() != "tcp" {
				conn.Close()
				continue
			}
			l.Info().
				Str("remote_peer", conn.RemoteAddr().String()).
				Msg("incoming connection from remote peer")
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
	for torrent := range c.storage.Iterator() {
		go c.trackerRequest(ctx, torrent, event, &wg)
	}
	wg.Wait()
}

func (c *Client) trackerRequest(ctx context.Context, torrent *TorrentFile, event event, wg *sync.WaitGroup) {
	defer wg.Done()
	url, err := torrent.buildTrackerURL(c.id, listenPort, event)
	l := log.Ctx(ctx).With().Str("url", url).Logger()
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to buid tracker url: %w", err)).
			Send()
		return
	}
	ctx, cancel := context.WithTimeout(ctx, c.httpClient.Timeout)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to create new http request: %w", err)).
			Send()
		return
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to send http request: %w", err)).
			Send()
		return
	}
	defer resp.Body.Close()
	if event == stopped {
		return // no need to decode response
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to read from body tracker url%w", err)).
			Send()
		return
	}

	buf := bytes.NewBuffer(b)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to decode bencode: %w", err)).
			Send()
		return
	}
	response := newTrackerResponse(torrent.InfoHash)
	err = response.unmarshal(decoded)

	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable unmarchal to response: %w", err)).
			Send()
		return
	}
	c.intervalsLock.Lock()
	c.intervals[torrent.InfoHash] = response.interval
	defer c.intervalsLock.Unlock()
	c.peersCh <- response.peers
}

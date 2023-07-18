package p2p

import (
	"bytes"
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"io"
	"net"
	"net/http"
	"sync"
	"time"
	"torrent/bencode"
	"torrent/p2p/download"
	"torrent/p2p/listener"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
)

const PeerIdSize = 20

type PeerID [PeerIdSize]byte

func (id PeerID) PeerId() [PeerIdSize]byte {
	return id
}

type Client struct {
	id                PeerID
	port              uint16
	announcer         Announcer
	storage           *storage.Storage
	peersCh           chan Peers
	pmsLock           sync.RWMutex
	pms               PeerManagers
	intervalsLock     sync.RWMutex
	intervals         map[torrent.Hash]time.Duration
	connCh            <-chan net.Conn
	dms               *download.Managers
	progressConnReads chan *ProgressConnRead
	progressSpeed     chan *ProgressSpeed
}

func NewClient(id PeerID, port uint16, storage *storage.Storage, announcer Announcer) *Client {
	return &Client{
		id:                id,
		port:              port,
		announcer:         announcer,
		storage:           storage,
		peersCh:           make(chan Peers, 1),
		pms:               make(PeerManagers, 0, 512),
		intervals:         make(map[torrent.Hash]time.Duration),
		progressConnReads: make(chan *ProgressConnRead, 512),
		progressSpeed:     make(chan *ProgressSpeed),
	}
}

func (c *Client) Run(ctx context.Context) error {
	lis := listener.NewListenerFromContext(ctx)
	connCh, err := lis.Listen(c.port)
	if err != nil {
		return fmt.Errorf("unable to start listener: %w", err)
	}
	defer lis.Close()
	c.connCh = connCh

	c.dms = download.CreateDownloadManagers(c.storage)
	c.trackerRequests(ctx, torrent.Started)

	var wg sync.WaitGroup // todo use err group
	wg.Add(2)
	go c.managePeers(ctx, &wg)
	go c.trackerRegularRequests(ctx, &wg)
	go c.calculateDownloadSpeed(ctx)
	wg.Wait()
	close(c.peersCh)
	// pass new ctx, because old one is already canceled
	c.trackerRequests(log.Ctx(ctx).WithContext(context.Background()), torrent.Stopped)
	_ = c.storage.Close()
	return ctx.Err()
}

func (c *Client) ProgressSpeed() <-chan *ProgressSpeed {
	return c.progressSpeed
}

func (c *Client) trackerRegularRequests(ctx context.Context, wg *sync.WaitGroup) {
	defer wg.Done()
	for t := range c.storage.Iterator() {
		t := t
		go func() {
			c.intervalsLock.RLock()
			ticker := time.NewTicker(c.intervals[t.InfoHash])
			c.intervalsLock.RUnlock()
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					wg.Add(1)
					c.trackerRequest(ctx, t, torrent.Regular, wg)
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
		case conn, ok := <-c.connCh:
			if !ok {
				return
			}
			addr := conn.RemoteAddr().(*net.TCPAddr)
			peer := Peer{
				InfoHash: torrent.Hash{},
				IP:       addr.IP,
				Port:     uint16(addr.Port),
				Conn:     conn,
			}
			pm := NewPeerManager(c.id, c.storage, peer, c.dms, c.progressConnReads)
			c.pmsLock.Lock()
			c.pms = append(c.pms, pm)
			c.pmsLock.Unlock()
			wg.Add(1)
			go pm.Run(ctx, wg)
		case peers, ok := <-c.peersCh:
			if !ok {
				return
			}
			c.pmsLock.Lock()
			for _, peer := range peers {
				foundPms := c.pms.FindByInfoHashAndIp(peer.InfoHash, peer.IP)
				if len(foundPms) == 0 {
					pm := NewPeerManager(c.id, c.storage, peer, c.dms, c.progressConnReads)
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

func (c *Client) trackerRequests(ctx context.Context, event torrent.Event) {
	var wg sync.WaitGroup
	wg.Add(c.storage.Len())
	for t := range c.storage.Iterator() {
		go c.trackerRequest(ctx, t, event, &wg)
	}
	wg.Wait()
}

func (c *Client) trackerRequest(ctx context.Context, t *torrent.File, event torrent.Event, wg *sync.WaitGroup) {
	defer wg.Done()
	url, err := t.BuildTrackerURL(c.id, c.port, event)
	l := log.Ctx(ctx).With().Str("url", url).Logger()
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to buid tracker url: %w", err)).
			Send()
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to create new http request: %w", err)).
			Send()
		return
	}
	resp, err := c.announcer.Do(req)
	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable to send http request: %w", err)).
			Send()
		return
	}
	defer resp.Body.Close()
	if event == torrent.Stopped {
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
	response := newTrackerResponse(t.InfoHash)
	err = response.unmarshal(decoded)

	if err != nil {
		l.Error().
			Err(fmt.Errorf("unable unmarchal to response: %w", err)).
			Send()
		return
	}
	c.intervalsLock.Lock()
	c.intervals[t.InfoHash] = response.interval
	defer c.intervalsLock.Unlock()
	c.peersCh <- response.peers
}

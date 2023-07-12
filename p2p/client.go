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
	"torrent/p2p/divide"
	listener2 "torrent/p2p/listener"
)

const PeerIdSize = 20
const BlockLen = 1 << 14 // 16kB

type PeerID [PeerIdSize]byte

type Client struct {
	id            PeerID
	port          uint16
	announcer     Announcer
	storage       *Storage
	peersCh       chan Peers
	pmsLock       sync.RWMutex
	pms           PeerManagers
	intervalsLock sync.RWMutex
	intervals     map[Hash]time.Duration
	connCh        <-chan net.Conn
	dms           map[Hash]*downloadManager
}

func NewClient(id PeerID, port uint16, storage *Storage, announcer Announcer) *Client {
	return &Client{
		id:        id,
		port:      port,
		announcer: announcer,
		storage:   storage,
		peersCh:   make(chan Peers, 1),
		pms:       make(PeerManagers, 0, 512),
		intervals: make(map[Hash]time.Duration),
		dms:       make(map[Hash]*downloadManager),
	}
}

func (c *Client) Run(ctx context.Context) error {
	listener := listener2.NewListenerFromContext(ctx)
	connCh, err := listener.Listen(c.port)
	if err != nil {
		return fmt.Errorf("unable to start listener: %w", err)
	}
	defer listener.Close()
	c.connCh = connCh

	c.trackerRequests(ctx, started)
	c.createDownloadTasks()

	var wg sync.WaitGroup
	wg.Add(2)
	go c.managePeers(ctx, &wg)
	go c.trackerRegularRequests(ctx, &wg)
	wg.Wait()
	close(c.peersCh)
	// pass new ctx, because old one is already canceled
	c.trackerRequests(log.Ctx(ctx).WithContext(context.Background()), stopped)
	_ = c.storage.Close()
	return ctx.Err()
}

func (c *Client) createDownloadTasks() {
	for torrent := range c.storage.Iterator() {
		torrent := torrent
		bitfield := c.storage.GetBitfield(torrent.InfoHash)
		blocks := divide.Divide(torrent.Length, []int{torrent.PieceLength, BlockLen})
		c.dms[torrent.InfoHash] = newDownloadManager(blocks, bitfield)
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
		case conn, ok := <-c.connCh:
			if !ok {
				return
			}
			addr := conn.RemoteAddr().(*net.TCPAddr)
			peer := Peer{
				InfoHash: Hash{},
				IP:       addr.IP,
				Port:     uint16(addr.Port),
				Conn:     conn,
			}
			pm := NewPeerManager(c.id, c.storage, peer, c.dms)
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
					pm := NewPeerManager(c.id, c.storage, peer, c.dms)
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
	url, err := torrent.buildTrackerURL(c.id, c.port, event)
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

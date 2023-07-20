package p2p

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"net"
	"sync"
	"time"
	"torrent/p2p/download"
	"torrent/p2p/listener"
	"torrent/p2p/peer"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
	"torrent/p2p/tracker"
)

type Client struct {
	id                peer.ID
	port              uint16
	storage           *storage.Storage
	peersCh           chan peer.Peers
	pmsLock           sync.RWMutex
	pms               PeerManagers
	connCh            <-chan net.Conn
	dms               *download.Managers
	progressConnReads chan *ProgressConnRead
	progressSpeed     chan *ProgressSpeed
	progressPieces    chan *ProgressPieceDownloaded
}

func NewClient(id peer.ID, port uint16, storage *storage.Storage) *Client {
	return &Client{
		id:                id,
		port:              port,
		storage:           storage,
		peersCh:           make(chan peer.Peers, storage.Len()),
		pms:               make(PeerManagers, 0, 512),
		progressConnReads: make(chan *ProgressConnRead, 512),
		progressPieces:    make(chan *ProgressPieceDownloaded, 512),
		progressSpeed:     make(chan *ProgressSpeed),
	}
}

func (c *Client) Run(ctx context.Context) (err error) {
	defer c.storage.Close()
	lis := listener.NewListenerFromContext(ctx)
	c.connCh, err = lis.Listen(c.port)
	if err != nil {
		return fmt.Errorf("unable to start listener: %w", err)
	}
	defer lis.Close()
	c.dms = download.CreateDownloadManagers(c.storage)
	c.sendInitialProgress()
	g, ctx := errgroup.WithContext(ctx)
	trackers := make([]*tracker.Tracker, 0, c.storage.Len())
	for file := range c.storage.Iterator() {
		t := tracker.New(file, c.id, c.port, c.peersCh)
		trackers = append(trackers, t)
		g.Go(func() error {
			return t.Run(ctx)
		})
	}
	g.Go(func() error {
		return c.managePeers(ctx)
	})
	go c.calculateDownloadSpeed(ctx)
	return g.Wait()
}

func (c *Client) ProgressSpeed() <-chan *ProgressSpeed {
	return c.progressSpeed
}

func (c *Client) ProgressPieces() <-chan *ProgressPieceDownloaded {
	return c.progressPieces
}

func (c *Client) sendInitialProgress() {
	for t := range c.storage.Iterator() {
		downloaded := c.storage.GetBitfield(t.InfoHash).DownloadedPiecesCount()
		c.progressPieces <- NewProgressPieceDownloaded(t.InfoHash, downloaded)
	}
}

func (c *Client) managePeers(ctx context.Context) error {
	wg := new(sync.WaitGroup)
	defer wg.Wait()

	l := log.Ctx(ctx)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case conn, ok := <-c.connCh:
			if !ok {
				return nil
			}
			addr := conn.RemoteAddr().(*net.TCPAddr)
			p := peer.Peer{
				InfoHash: torrent.Hash{},
				IP:       addr.IP,
				Port:     uint16(addr.Port),
				Conn:     conn,
			}
			pm := NewPeerManager(c.id, c.storage, p, c.dms, c.progressConnReads, c.progressPieces)
			c.pmsLock.Lock()
			c.pms = append(c.pms, pm)
			c.pmsLock.Unlock()
			wg.Add(1)
			go pm.Run(ctx, wg)
		case peers, ok := <-c.peersCh:
			if !ok {
				return nil
			}
			c.pmsLock.Lock()
			for _, p := range peers {
				foundPms := c.pms.FindByInfoHashAndIp(p.InfoHash, p.IP)
				if len(foundPms) == 0 {
					pm := NewPeerManager(c.id, c.storage, p, c.dms, c.progressConnReads, c.progressPieces)
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
			return ctx.Err()
		}
	}
}

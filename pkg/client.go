package pkg

import (
	"context"
	"fmt"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"net"
	"sync"
	"time"

	"github.com/mineroot/torrent/pkg/download"
	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/pkg/listener"
	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
	"github.com/mineroot/torrent/pkg/tracker"
)

type Client struct {
	id                peer.ID
	port              uint16
	storage           *storage.Storage
	peersCh           chan peer.Peers
	pmsLock           sync.RWMutex
	pms               peer.Managers
	connCh            <-chan net.Conn
	dms               *download.Managers
	progressConnReads chan *event.ProgressConnRead
	progressSpeed     chan *event.ProgressSpeed
	progressPieces    chan *event.ProgressPieceDownloaded
}

func NewClient(id peer.ID, port uint16, storage *storage.Storage) *Client {
	return &Client{
		id:                id,
		port:              port,
		storage:           storage,
		peersCh:           make(chan peer.Peers, storage.Len()),
		pms:               make(peer.Managers, 0, 512),
		progressConnReads: make(chan *event.ProgressConnRead, 512),
		progressPieces:    make(chan *event.ProgressPieceDownloaded, 512),
		progressSpeed:     make(chan *event.ProgressSpeed),
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

func (c *Client) ProgressSpeed() <-chan *event.ProgressSpeed {
	return c.progressSpeed
}

func (c *Client) ProgressPieces() <-chan *event.ProgressPieceDownloaded {
	return c.progressPieces
}

func (c *Client) sendInitialProgress() {
	for t := range c.storage.Iterator() {
		downloaded := c.storage.GetBitfield(t.InfoHash).DownloadedPiecesCount()
		c.progressPieces <- event.NewProgressPieceDownloaded(t.InfoHash, downloaded)
	}
}

func (c *Client) managePeers(ctx context.Context) error {
	g := new(errgroup.Group)
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
			pm := peer.NewManager(c.id, &net.Dialer{}, c.storage, p, c.dms, c.progressConnReads, c.progressPieces)
			c.pmsLock.Lock()
			c.pms = append(c.pms, pm)
			c.pmsLock.Unlock()
			g.Go(func() error {
				return pm.Run(ctx)
			})
		case peers, ok := <-c.peersCh:
			if !ok {
				return nil
			}
			c.pmsLock.Lock()
			for _, p := range peers {
				foundPms := c.pms.FindByInfoHashAndIp(p.InfoHash, p.IP)
				if len(foundPms) == 0 {
					pm := peer.NewManager(c.id, &net.Dialer{}, c.storage, p, c.dms, c.progressConnReads, c.progressPieces)
					c.pms = append(c.pms, pm)
					g.Go(func() error {
						return pm.Run(ctx)
					})
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
			_ = g.Wait()
			return ctx.Err()
		}
	}
}

package peer

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/download"
	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
	"github.com/mineroot/torrent/utils"
)

const (
	handshakeLen = 68
	pstr         = "BitTorrent protocol"
)

type Manager struct {
	clientId          ID
	storage           storage.Reader
	peer              Peer
	isAlive           atomic.Bool
	incomeMessagesCh  chan *Message
	outcomeMessageCh  chan *Message
	amChoking         atomic.Bool
	amInterested      atomic.Bool
	peerChoking       atomic.Bool
	peerInterested    atomic.Bool
	bitfieldReceived  chan struct{}
	peerBitfield      *bitfield.Bitfield
	myBitfield        *bitfield.Bitfield
	log               zerolog.Logger
	torrent           *torrent.File
	dm                *download.Manager
	file              afero.File
	dms               *download.Managers
	progressConnReads chan<- *event.ProgressConnRead
	progressPieces    chan<- *event.ProgressPieceDownloaded
	requestedBlocks   *download.BlocksSyncMap
}

func NewManager(
	clientId ID,
	storage storage.Reader,
	peer Peer,
	dms *download.Managers,
	progressConnReads chan<- *event.ProgressConnRead,
	progressPieces chan<- *event.ProgressPieceDownloaded,
) *Manager {
	pm := &Manager{
		clientId:          clientId,
		storage:           storage,
		peer:              peer,
		incomeMessagesCh:  make(chan *Message, 512),
		outcomeMessageCh:  make(chan *Message, 512),
		bitfieldReceived:  make(chan struct{}),
		dms:               dms,
		progressConnReads: progressConnReads,
		progressPieces:    progressPieces,
		requestedBlocks:   download.NewBlocksSyncMap(),
	}
	pm.isAlive.Store(true)
	pm.amChoking.Store(true)
	pm.peerChoking.Store(true)
	return pm
}

func (pm *Manager) GetHash() torrent.Hash {
	return pm.peer.InfoHash
}

func (pm *Manager) Run(ctx context.Context, wg *sync.WaitGroup) {
	defer func() {
		wg.Done()
		pm.isAlive.Store(false)
	}()
	if !pm.isAlive.Load() {
		panic("peer manager is not alive")
	}
	pm.setLoggerFromCtx(ctx)

	var err error
	if pm.peer.Conn == nil {
		// we initiate connection and send a handshake first
		if err = pm.peer.sendHandshake(pm.clientId); err != nil {
			err = fmt.Errorf("unable to send handshake too remote peer: %w", err)
		}
		pm.torrent = pm.storage.Get(pm.peer.InfoHash)
	} else {
		// remote peer connected to us, and we are waiting for a handshake
		if pm.torrent, err = pm.peer.acceptHandshake(pm.clientId, pm.storage); err != nil {
			err = fmt.Errorf("unable to accept handshake from remote peer: %w", err)
		}
	}
	if err != nil {
		pm.log.Error().Err(err).Send()
		return
	}
	pm.log.Info().Msg("successful handshake")

	pm.file = pm.storage.GetFile(pm.peer.InfoHash)
	pm.dm = pm.dms.Load(pm)
	pm.myBitfield = pm.storage.GetBitfield(pm.peer.InfoHash)

	// todo send this synchronous
	_ = pm.sendMessage(ctx, NewBitfield(pm.myBitfield))
	_ = pm.sendMessage(ctx, NewUnChoke())
	time.Sleep(time.Second)
	_ = pm.sendMessage(ctx, NewInterested())

	g, ctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return pm.readMessages()
	})
	g.Go(func() error {
		return pm.writeMessages(ctx)
	})
	g.Go(func() error {
		return pm.handleMessages(ctx)
	})
	g.Go(func() error {
		return pm.download(ctx)
	})
	g.Go(func() error {
		return pm.kill(ctx)
	})

	if err = g.Wait(); err != nil {
		pm.log.Error().Err(err).Msg("peer manager is dying...")
	}
}

func (pm *Manager) IsAlive() bool {
	return pm.isAlive.Load()
}

func (pm *Manager) kill(ctx context.Context) error {
	<-ctx.Done()
	pm.isAlive.Store(false)
	if pm.peer.Conn != nil {
		pm.peer.Conn.Close()
	}
	return nil
}

func (pm *Manager) readMessages() error {
	defer close(pm.incomeMessagesCh)
	_ = pm.peer.Conn.SetReadDeadline(time.Time{})
	for {
		bytesRead := 0
		bufLen := make([]byte, 4)
		n, err := io.ReadFull(pm.peer.Conn, bufLen)
		if err != nil {
			return err
		}
		bytesRead += n
		msgLen := binary.BigEndian.Uint32(bufLen)
		if msgLen == 0 { //keep-alive message
			pm.progressConnReads <- event.NewProgressConnRead(pm.GetHash(), bytesRead)
			continue
		}
		msgBuf := make([]byte, msgLen)
		n, err = io.ReadFull(pm.peer.Conn, msgBuf)
		if err != nil {
			return err
		}
		bytesRead += n
		pm.progressConnReads <- event.NewProgressConnRead(pm.GetHash(), bytesRead)
		pm.incomeMessagesCh <- &Message{
			ID:      messageId(msgBuf[0]),
			Payload: msgBuf[1:],
		}
	}
}

func (pm *Manager) sendMessage(ctx context.Context, message *Message) error {
	// allow to send msgUnChoke and msgBitfield and msgInterested even if amChoking
	if pm.amChoking.Load() && !(message.ID == msgBitfield || message.ID == msgUnChoke || message.ID == msgInterested) {
		return fmt.Errorf("i am choking")
	}
	pm.log.Debug().
		Int("messageId", int(message.ID)).Str("payload_len", utils.FormatBytes(uint(len(message.Payload)))).
		Msg("msg sent")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case pm.outcomeMessageCh <- message:
		return nil
	}
}

func (pm *Manager) writeMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-pm.outcomeMessageCh:
			if !ok {
				return nil
			}
			_ = pm.peer.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			_, err := pm.peer.Conn.Write(message.Encode())
			if err != nil {
				return err
			}
		}
	}
}

func (pm *Manager) download(ctx context.Context) error {
	// wait for bitfield
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pm.bitfieldReceived:
	}

	const minGrowFactor = 1
	growFactor := 4
	growFunc := func(x int) int {
		if x < 5 {
			return x * x
		}
		return 5 * x
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		if pm.amChoking.Load() {
			time.Sleep(time.Second)
			continue
		}

		blocksToRequest := growFunc(growFactor)
		pm.log.Debug().Int("blocks_to_request", blocksToRequest).Send()
		i := 0
		for {
			if i >= blocksToRequest {
				break
			}
			block, err := pm.dm.GenerateBlock(ctx)
			if errors.Is(err, download.ErrNoMoreBlocks) {
				pm.log.Info().Int("requested_messages_left", pm.requestedBlocks.Len()).Msg("download completed")
				for block := range pm.requestedBlocks.Iterate() {
					message := NewCancel(uint32(block.PieceIndex), uint32(block.Begin), uint32(block.Len))
					_ = pm.sendMessage(ctx, message)
				}
				return nil
			}
			if err != nil {
				return err
			}
			if !pm.myBitfield.Has(block.PieceIndex) && pm.peerBitfield.Has(block.PieceIndex) && !pm.requestedBlocks.Has(block) {
				// if we don't have a piece and remote peer has a piece
				// and we not just yet requested block, then request block
				message := NewRequest(uint32(block.PieceIndex), uint32(block.Begin), uint32(block.Len))
				if err = pm.sendMessage(ctx, message); err != nil {
					break
				}
				// add block to map for tracking requested blocks
				pm.requestedBlocks.Add(block)
				i++
			}
		}
		// wait 1 sec
		<-ticker.C
		// if block isn't received in 5 seconds, we assume it never will be received
		remainingRequests := pm.requestedBlocks.LenNonExpired(5 * time.Second)
		// adjust growFactor based on non-expired blocks count
		if remainingRequests > 0 {
			growFactor--
		} else {
			growFactor++
		}
		if growFactor < minGrowFactor {
			growFactor = minGrowFactor
		}
	}
}

func (pm *Manager) handleMessages(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case message, ok := <-pm.incomeMessagesCh:
			if !ok {
				return nil
			}
			pm.log.Debug().
				Int("messageId", int(message.ID)).Str("payload_len", utils.FormatBytes(uint(len(message.Payload)))).
				Msg("msg received")

			handler, ok := messageHandlers[message.ID]
			if !ok {
				pm.log.Warn().Int("message_id", int(message.ID)).Bytes("payload", message.Payload).Msg("unknown message id")
				break
			}
			if err := handler(ctx, pm, message); err != nil {
				return nil
			}
		}
	}
}

func (pm *Manager) setLoggerFromCtx(ctx context.Context) {
	pm.log = log.Ctx(ctx).With().Str("peer_id", string(pm.clientId[:])).Str("remote_peer", pm.peer.Address()).Logger().
		Hook(zerolog.HookFunc(func(e *zerolog.Event, _ zerolog.Level, _ string) {
			if !pm.peer.InfoHash.IsZero() {
				e.Str("info_hash", pm.peer.InfoHash.String())
			}
		}))
}

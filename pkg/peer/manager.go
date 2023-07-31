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
	*managerState
	once                     sync.Once
	clientId                 ID
	dialer                   ContextDialer
	storage                  storage.Reader
	peer                     Peer
	incomeMessagesCh         chan *Message
	outcomeMessageCh         chan *Message
	bitfieldReceivedOnce     sync.Once
	bitfieldReceived         chan struct{}
	firstRequestReceivedOnce sync.Once
	firstRequestReceived     chan struct{}
	peerBitfield             *bitfield.Bitfield
	myBitfield               *bitfield.Bitfield
	log                      zerolog.Logger
	torrent                  *torrent.File
	bgs                      *download.BlockGenerators
	bg                       *download.BlockGenerator
	file                     afero.File
	progressConnReads        chan<- *event.ProgressConnRead
	progressPieces           chan<- *event.ProgressPieceDownloaded
	myRequestedBlocks        *download.BlocksSyncMap
	peerRequestedBlocks      *download.BlocksSyncMap
}

func NewManager(
	clientId ID,
	dialer ContextDialer,
	storage storage.Reader,
	peer Peer,
	bgs *download.BlockGenerators,
	progressConnReads chan<- *event.ProgressConnRead,
	progressPieces chan<- *event.ProgressPieceDownloaded,
) *Manager {
	pm := &Manager{
		clientId:             clientId,
		dialer:               dialer,
		storage:              storage,
		peer:                 peer,
		managerState:         newManagerState(),
		incomeMessagesCh:     make(chan *Message, 512),
		outcomeMessageCh:     make(chan *Message, 512),
		bitfieldReceived:     make(chan struct{}),
		firstRequestReceived: make(chan struct{}),
		bgs:                  bgs,
		progressConnReads:    progressConnReads,
		progressPieces:       progressPieces,
		myRequestedBlocks:    download.NewBlocksSyncMap(),
		peerRequestedBlocks:  download.NewBlocksSyncMap(),
	}
	return pm
}

func (pm *Manager) Run(ctx context.Context) (err error) {
	pm.once.Do(func() {
		defer pm.isAlive.Store(false)
		pm.setLoggerFromCtx(ctx)
		err = pm.run(ctx)
		pm.log.Error().Err(err).Msg("peer manager is dying...")
	})
	return
}

func (pm *Manager) run(ctx context.Context) (err error) {
	g, ctx := errgroup.WithContext(ctx)
	go func() {
		<-ctx.Done()
		if pm.peer.Conn != nil {
			pm.peer.Conn.Close()
		}
	}()

	if pm.peer.Conn == nil {
		// we initiate connection and send a handshake first
		if err = pm.peer.sendHandshake(ctx, pm.dialer, pm.clientId); err != nil {
			return fmt.Errorf("unable to send handshake too remote peer: %w", err)
		}
		pm.torrent = pm.storage.Get(pm.peer.InfoHash)
	} else {
		// remote peer connected to us, and we are waiting for a handshake
		if pm.torrent, err = pm.peer.acceptHandshake(ctx, pm.storage, pm.clientId); err != nil {
			return fmt.Errorf("unable to accept handshake from remote peer: %w", err)
		}
	}
	pm.log.Info().Msg("successful handshake")

	pm.file = pm.storage.GetFile(pm.peer.InfoHash)
	pm.bg = pm.bgs.Load(pm)
	pm.myBitfield = pm.storage.GetBitfield(pm.peer.InfoHash)

	// todo send this synchronous
	_ = pm.sendMessage(ctx, NewBitfield(pm.myBitfield))
	_ = pm.sendMessage(ctx, NewUnChoke())
	time.Sleep(time.Second)
	_ = pm.sendMessage(ctx, NewInterested())

	g.Go(pm.readMessages)
	g.Go(utils.WithContext(ctx, pm.writeMessages))
	g.Go(utils.WithContext(ctx, pm.handleMessages))
	g.Go(utils.WithContext(ctx, pm.download))
	g.Go(utils.WithContext(ctx, pm.upload))
	return g.Wait()
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
			block, err := pm.bg.Generate(ctx)
			if errors.Is(err, download.ErrNoMoreBlocks) {
				pm.log.Info().Int("requested_messages_left", pm.myRequestedBlocks.Len()).Msg("download completed")
				for block := range pm.myRequestedBlocks.Iterate() {
					message := NewCancel(block)
					_ = pm.sendMessage(ctx, message)
				}
				return nil
			}
			if err != nil {
				return err
			}
			if !pm.myBitfield.Has(block.PieceIndex) && pm.peerBitfield.Has(block.PieceIndex) && !pm.myRequestedBlocks.Has(block) {
				// if we don't have a piece but remote peer has a piece
				// and we not just yet requested block, then request block
				message := NewRequest(block)
				if err = pm.sendMessage(ctx, message); err != nil {
					break
				}
				// add block to map for tracking requested blocks
				pm.myRequestedBlocks.Add(block)
				i++
			}
		}
		// wait 1 sec
		<-ticker.C
		// if a block isn't received in 5 seconds, we assume it never will be received
		remainingRequests := pm.myRequestedBlocks.LenNonExpired(5 * time.Second)
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

func (pm *Manager) upload(ctx context.Context) error {
	// wait for the first 'request' message from peer
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-pm.firstRequestReceived:
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			for block := range pm.peerRequestedBlocks.Iterate() {
				// if block's request was canceled by peer
				if !pm.peerRequestedBlocks.Has(block) {
					continue
				}
				offset := block.PieceIndex*pm.torrent.PieceLength + block.Begin
				if offset+block.Len > pm.torrent.Length {
					block.Len = pm.torrent.Length - offset
				}
				buf := make([]byte, block.Len)
				_, err := pm.file.ReadAt(buf, int64(offset))
				if err != nil {
					return err
				}
				if err = pm.sendMessage(ctx, NewPiece(block, buf)); err != nil {
					continue
				}
				pm.peerRequestedBlocks.Delete(block)
			}
		}
	}
}

func (pm *Manager) handleMessages(ctx context.Context) error {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
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
			//go func() { // TODO: fucking data race in afero
			if err := handler(pm, message); err != nil {
				cancel(err)
			}
			//}()
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

func (pm *Manager) GetHash() torrent.Hash {
	return pm.peer.InfoHash
}

func (pm *Manager) IsAlive() bool {
	return pm.isAlive.Load()
}

type managerState struct {
	isAlive        atomic.Bool
	amChoking      atomic.Bool
	amInterested   atomic.Bool
	peerChoking    atomic.Bool
	peerInterested atomic.Bool
}

func newManagerState() *managerState {
	state := &managerState{}
	state.isAlive.Store(true)
	state.amChoking.Store(true)
	state.peerChoking.Store(true)
	return state
}

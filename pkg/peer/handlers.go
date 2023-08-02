package peer

import (
	"context"
	"encoding/binary"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/download"
	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/utils"
)

type messageHandler func(context.Context, *Manager, *Message) error

var messageHandlers = map[messageId]messageHandler{
	msgChoke:         msgChokeHandler,
	msgUnChoke:       msgUnChokeHandler,
	msgInterested:    msgInterestedHandler,
	msgNotInterested: msgNotInterestedHandler,
	msgHave:          msgHaveHandler,
	msgBitfield:      msgBitfieldHandler,
	msgRequest:       msgRequestHandler,
	msgPiece:         msgPieceHandler,
	msgCancel:        msgCancelHandler,
	msgPort:          noopHandler,
}

func msgChokeHandler(_ context.Context, pm *Manager, _ *Message) error {
	pm.amChoking.Store(true)
	return nil
}

func msgUnChokeHandler(_ context.Context, pm *Manager, _ *Message) error {
	pm.amChoking.Store(false)
	return nil
}

func msgInterestedHandler(_ context.Context, pm *Manager, _ *Message) error {
	pm.peerInterested.Store(true)
	return nil
}

func msgNotInterestedHandler(_ context.Context, pm *Manager, _ *Message) error {
	pm.peerInterested.Store(false)
	return nil
}

func msgHaveHandler(ctx context.Context, pm *Manager, message *Message) error {
	index := binary.BigEndian.Uint32(message.Payload[:4])
	// ignoring if peer sent the wrong piece index
	_ = pm.peerBitfield.Set(int(index))
	pm.checkAmInterested(ctx)
	return nil
}

func msgBitfieldHandler(_ context.Context, pm *Manager, message *Message) error {
	var err error
	pm.bitfieldReceivedOnce.Do(func() {
		var bf *bitfield.Bitfield
		bf, err = bitfield.FromPayload(message.Payload, pm.td.Torrent().PiecesCount())
		if err != nil {
			return
		}
		pm.peerBitfield = bf
		close(pm.bitfieldReceived)
	})
	return err
}

func msgRequestHandler(_ context.Context, pm *Manager, message *Message) error {
	pm.firstRequestReceivedOnce.Do(func() {
		close(pm.firstRequestReceived)
	})
	index := binary.BigEndian.Uint32(message.Payload[:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	length := binary.BigEndian.Uint32(message.Payload[8:12])
	block := download.NewBlock(int(index), int(begin), int(length))
	pm.peerRequestedBlocks.Add(block)
	return nil
}

func msgPieceHandler(_ context.Context, pm *Manager, message *Message) error {
	index := int(binary.BigEndian.Uint32(message.Payload[:4]))
	// discard block if we already have its piece
	if pm.td.Bitfield().Has(index) {
		pm.log.Warn().Int("piece", index).Msg("block discarded")
		return nil
	}
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	data := message.Payload[8:]
	offset := index*pm.td.Torrent().PieceLength + int(begin)
	if _, err := pm.td.WriteAt(data, int64(offset)); err != nil {
		return err
	}

	block := download.NewBlock(index, int(begin), len(data))
	isVerified, err := pm.bg.MarkAsDownloaded(block, pm.td, pm.td.Torrent().PieceHashes[index], pm.td.Torrent().PieceLength)
	if err != nil {
		return err
	}
	// delete block from requested after receiving it
	pm.myRequestedBlocks.Delete(block)
	if isVerified {
		pm.progressPieces <- event.NewProgressPieceDownloaded(pm.td.Torrent().InfoHash, pm.td.Bitfield().DownloadedPiecesCount())
	}
	pm.log.Debug().Int("piece", index).Str("len", utils.FormatBytes(uint(len(data)))).Msg("block downloaded")
	return nil
}

func msgCancelHandler(_ context.Context, pm *Manager, message *Message) error {
	index := binary.BigEndian.Uint32(message.Payload[:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	length := binary.BigEndian.Uint32(message.Payload[8:12])
	block := download.NewBlock(int(index), int(begin), int(length))
	pm.peerRequestedBlocks.Delete(block)
	return nil
}

func noopHandler(context.Context, *Manager, *Message) error {
	return nil
}

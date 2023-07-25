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
	msgHave:          noopHandler, // TODO
	msgBitfield:      msgBitfieldHandler,
	msgRequest:       msgRequestHandler,
	msgPiece:         msgPieceHandler,
	msgCancel:        noopHandler, // TODO
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

func msgBitfieldHandler(_ context.Context, pm *Manager, message *Message) error {
	bf, err := bitfield.FromPayload(message.Payload, pm.torrent.PiecesCount())
	if err != nil {
		return err
	}
	pm.peerBitfield = bf
	pm.bitfieldReceived <- struct{}{}
	return nil
}

func msgRequestHandler(ctx context.Context, pm *Manager, message *Message) error {
	index := binary.BigEndian.Uint32(message.Payload[:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	length := binary.BigEndian.Uint32(message.Payload[8:12])
	offset := int(index)*pm.torrent.PieceLength + int(begin)
	if offset+int(length) > pm.torrent.Length {
		length = uint32(pm.torrent.Length - offset)
	}
	buf := make([]byte, length)
	_, err := pm.file.ReadAt(buf, int64(offset))
	if err != nil {
		return err
	}
	_ = pm.sendMessage(ctx, NewPiece(index, begin, buf))
	return nil
}

func msgPieceHandler(_ context.Context, pm *Manager, message *Message) error {
	index := int(binary.BigEndian.Uint32(message.Payload[:4]))
	// discard block if we already have its piece
	if pm.myBitfield.Has(index) {
		pm.log.Warn().Int("piece", index).Msg("block discarded")
		return nil
	}
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	data := message.Payload[8:]
	offset := index*pm.torrent.PieceLength + int(begin)
	if _, err := pm.file.WriteAt(data, int64(offset)); err != nil {
		return err
	}

	block := download.Block{
		PieceIndex: index,
		Begin:      int(begin),
		Len:        len(data),
	}
	isVerified, err := pm.dm.MarkAsDownloaded(block, pm.file, pm.torrent.PieceHashes[index], pm.torrent.PieceLength)
	if err != nil {
		return err
	}
	// delete block from requested after receiving it
	pm.requestedBlocks.Delete(block)
	if isVerified {
		pm.progressPieces <- event.NewProgressPieceDownloaded(pm.torrent.InfoHash, pm.myBitfield.DownloadedPiecesCount())
	}
	pm.log.Debug().Int("piece", index).Str("len", utils.FormatBytes(uint(len(data)))).Msg("block downloaded")
	return nil
}

func noopHandler(context.Context, *Manager, *Message) error {
	return nil
}

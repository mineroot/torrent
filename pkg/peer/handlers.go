package peer

import (
	"encoding/binary"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/download"
	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/utils"
)

type messageHandler func(*Manager, *Message) error

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

func msgChokeHandler(pm *Manager, _ *Message) error {
	pm.amChoking.Store(true)
	return nil
}

func msgUnChokeHandler(pm *Manager, _ *Message) error {
	pm.amChoking.Store(false)
	return nil
}

func msgInterestedHandler(pm *Manager, _ *Message) error {
	pm.peerInterested.Store(true)
	return nil
}

func msgNotInterestedHandler(pm *Manager, _ *Message) error {
	pm.peerInterested.Store(false)
	return nil
}

func msgHaveHandler(pm *Manager, message *Message) error {
	index := binary.BigEndian.Uint32(message.Payload[:4])
	// ignoring if peer sent the wrong piece index
	_ = pm.peerBitfield.Set(int(index))
	return nil
}

func msgBitfieldHandler(pm *Manager, message *Message) error {
	var err error
	pm.bitfieldReceivedOnce.Do(func() {
		var bf *bitfield.Bitfield
		bf, err = bitfield.FromPayload(message.Payload, pm.torrent.PiecesCount())
		if err != nil {
			return
		}
		pm.peerBitfield = bf
		close(pm.bitfieldReceived)
	})
	return err
}

func msgRequestHandler(pm *Manager, message *Message) error {
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

func msgPieceHandler(pm *Manager, message *Message) error {
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

	block := download.NewBlock(index, int(begin), len(data))
	isVerified, err := pm.bg.MarkAsDownloaded(block, pm.file, pm.torrent.PieceHashes[index], pm.torrent.PieceLength)
	if err != nil {
		return err
	}
	// delete block from requested after receiving it
	pm.myRequestedBlocks.Delete(block)
	if isVerified {
		pm.progressPieces <- event.NewProgressPieceDownloaded(pm.torrent.InfoHash, pm.myBitfield.DownloadedPiecesCount())
	}
	pm.log.Debug().Int("piece", index).Str("len", utils.FormatBytes(uint(len(data)))).Msg("block downloaded")
	return nil
}

func msgCancelHandler(pm *Manager, message *Message) error {
	index := binary.BigEndian.Uint32(message.Payload[:4])
	begin := binary.BigEndian.Uint32(message.Payload[4:8])
	length := binary.BigEndian.Uint32(message.Payload[8:12])
	block := download.NewBlock(int(index), int(begin), int(length))
	pm.peerRequestedBlocks.Delete(block)
	return nil
}

func noopHandler(*Manager, *Message) error {
	return nil
}

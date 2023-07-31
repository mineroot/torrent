package peer

import (
	"encoding/binary"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/download"
)

type messageId int

const (
	msgChoke         messageId = 0
	msgUnChoke                 = 1
	msgInterested              = 2
	msgNotInterested           = 3
	msgHave                    = 4
	msgBitfield                = 5
	msgRequest                 = 6
	msgPiece                   = 7
	msgCancel                  = 8
	msgPort                    = 9
)

type Message struct {
	ID      messageId
	Payload []byte
}

func (m *Message) Encode() []byte {
	const lenPrefixSize = 4
	const messageIdSize = 1
	bufLen := lenPrefixSize + messageIdSize + len(m.Payload)
	buf := make([]byte, bufLen)
	messageLen := messageIdSize + len(m.Payload)
	binary.BigEndian.PutUint32(buf, uint32(messageLen))
	buf[lenPrefixSize] = byte(m.ID)
	copy(buf[lenPrefixSize+messageIdSize:], m.Payload)
	return buf
}

func NewUnChoke() *Message {
	return &Message{
		ID: msgUnChoke,
	}
}

func NewInterested() *Message {
	return &Message{
		ID: msgInterested,
	}
}

func NewBitfield(bf *bitfield.Bitfield) *Message {
	return &Message{
		ID:      msgBitfield,
		Payload: bf.Bitfield(),
	}
}

func NewRequest(block download.Block) *Message {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[:4], uint32(block.PieceIndex))
	binary.BigEndian.PutUint32(buf[4:8], uint32(block.Begin))
	binary.BigEndian.PutUint32(buf[8:12], uint32(block.Len))
	return &Message{
		ID:      msgRequest,
		Payload: buf,
	}
}

func NewPiece(block download.Block, data []byte) *Message {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[:4], uint32(block.PieceIndex))
	binary.BigEndian.PutUint32(buf[4:8], uint32(block.Begin))
	buf = append(buf, data...)
	return &Message{
		ID:      msgPiece,
		Payload: buf,
	}
}

func NewCancel(block download.Block) *Message {
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[:4], uint32(block.PieceIndex))
	binary.BigEndian.PutUint32(buf[4:8], uint32(block.Begin))
	binary.BigEndian.PutUint32(buf[8:12], uint32(block.Len))
	return &Message{
		ID:      msgCancel,
		Payload: buf,
	}
}

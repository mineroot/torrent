package p2p

import (
	"encoding/binary"
	"torrent/p2p/bitfield"
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

func NewRequest(pieceIndex, begin, length uint32) *Message { // todo make block as parameter
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[:4], pieceIndex)
	binary.BigEndian.PutUint32(buf[4:8], begin)
	binary.BigEndian.PutUint32(buf[8:12], length)
	return &Message{
		ID:      msgRequest,
		Payload: buf,
	}
}

func NewCancel(pieceIndex, begin, length uint32) *Message { // todo make block as parameter
	buf := make([]byte, 12)
	binary.BigEndian.PutUint32(buf[:4], pieceIndex)
	binary.BigEndian.PutUint32(buf[4:8], begin)
	binary.BigEndian.PutUint32(buf[8:12], length)
	return &Message{
		ID:      msgCancel,
		Payload: buf,
	}
}

func NewPiece(pieceIndex, begin uint32, data []byte) *Message {
	buf := make([]byte, 8)
	binary.BigEndian.PutUint32(buf[:4], pieceIndex)
	binary.BigEndian.PutUint32(buf[4:8], begin)
	buf = append(buf, data...)
	return &Message{
		ID:      msgPiece,
		Payload: buf,
	}
}

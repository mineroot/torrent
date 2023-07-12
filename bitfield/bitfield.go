package bitfield

import (
	"fmt"
	"sync"
)

const bits = 8

var ErrMalformedBitfield = fmt.Errorf("malformed bitfield")

type Bitfield struct {
	piecesCount int
	lock        sync.RWMutex
	bitfield    []byte
}

func (bf *Bitfield) BitfieldSize() int {
	return len(bf.bitfield)
}

func New(piecesCount int) *Bitfield {
	bitfieldSize := piecesCount / bits
	if piecesCount%bits != 0 {
		bitfieldSize++
	}
	return &Bitfield{
		piecesCount: piecesCount,
		bitfield:    make([]byte, bitfieldSize),
	}
}

func FromPayload(payload []byte, piecesCount int) (*Bitfield, error) {
	bitfieldSize := piecesCount / bits
	if piecesCount%bits != 0 {
		bitfieldSize++
	}
	if bitfieldSize != len(payload) {
		return nil, ErrMalformedBitfield
	}
	b := make([]byte, bitfieldSize)
	copy(b, payload)

	// spare bits should always be 0
	if piecesCount%bits != 0 {
		spareBitsCount := bits - piecesCount%bits
		lastByte := b[len(b)-1]
		if lastByte^byte((1<<spareBitsCount)-1) != 0xFF {
			return nil, ErrMalformedBitfield
		}
	}

	return &Bitfield{
		piecesCount: piecesCount,
		bitfield:    b,
	}, nil
}

func (bf *Bitfield) Bitfield() []byte {
	buf := make([]byte, len(bf.bitfield))
	copy(buf, bf.bitfield)
	return buf
}

func (bf *Bitfield) IsCompleted() bool {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	hasSpareBytes := bf.piecesCount%bits != 0
	for i, b := range bf.bitfield {
		val := byte(0xFF)
		isLastByte := i == len(bf.bitfield)-1
		if hasSpareBytes && isLastByte {
			lastBytePiecesCount := bf.piecesCount % bits
			val = val>>lastBytePiecesCount ^ val
		}
		if b != val {
			return false
		}
	}
	return true
}

func (bf *Bitfield) SetPiece(pieceIndex int) {
	byteIndex := pieceIndex / bits
	if pieceIndex >= bf.piecesCount {
		panic(fmt.Errorf("pieceIndex is out of range [0, %d)", bf.piecesCount))
	}
	bitIndex := pieceIndex % bits
	bf.lock.Lock()
	defer bf.lock.Unlock()
	bf.bitfield[byteIndex] |= 1 << (7 - bitIndex)
}

func (bf *Bitfield) Has(pieceIndex int) bool {
	byteIndex := pieceIndex / bits
	bitIndex := pieceIndex % bits
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	if pieceIndex > bf.piecesCount-1 {
		panic("pieceIndex out of range")
	}
	byteValue := bf.bitfield[byteIndex]
	mask := byte(1 << (7 - bitIndex))
	return (byteValue & mask) != 0
}

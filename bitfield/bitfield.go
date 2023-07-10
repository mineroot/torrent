package bitfield

import (
	"fmt"
	"sync"
)

const bits = 8

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

func (bf *Bitfield) Bitfield() []byte {
	buf := make([]byte, len(bf.bitfield))
	copy(buf, bf.bitfield)
	return buf
}

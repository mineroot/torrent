package bitfield

import (
	"bytes"
	"fmt"
	"sync"
)

const bits = 8

var ErrMalformedBitfield = fmt.Errorf("malformed bitfield")

type Bitfield struct {
	piecesCount       int
	lock              sync.RWMutex
	bitfield          []byte
	completedBitfield []byte
}

func New(piecesCount int) *Bitfield {
	bitfieldSize := piecesCount / bits
	if piecesCount%bits != 0 {
		bitfieldSize++
	}
	bitfield := Bitfield{
		piecesCount:       piecesCount,
		bitfield:          make([]byte, bitfieldSize),
		completedBitfield: make([]byte, bitfieldSize),
	}
	bitfield.initCompletedBitfield()
	return &bitfield
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
		if lastByte&byte((1<<spareBitsCount)-1) != 0 {
			return nil, ErrMalformedBitfield
		}
	}

	bitfield := &Bitfield{
		piecesCount:       piecesCount,
		bitfield:          b,
		completedBitfield: make([]byte, bitfieldSize),
	}
	bitfield.initCompletedBitfield()
	return bitfield, nil
}

func (bf *Bitfield) BitfieldSize() int {
	return len(bf.bitfield)
}

func (bf *Bitfield) PiecesCount() int {
	return bf.piecesCount
}

func (bf *Bitfield) DownloadedPiecesCount() int {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	downloadedTotal := 0
	for _, b := range bf.bitfield {
		downloadedTotal += bf.countSetBits(b)
	}
	return downloadedTotal
}

func (bf *Bitfield) Bitfield() []byte {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	buf := make([]byte, len(bf.bitfield))
	copy(buf, bf.bitfield)
	return buf
}

func (bf *Bitfield) IsCompleted() bool {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	return bytes.Equal(bf.bitfield, bf.completedBitfield)
}

func (bf *Bitfield) Set(pieceIndex int) error {
	bf.lock.Lock()
	defer bf.lock.Unlock()
	byteIndex := pieceIndex / bits
	if pieceIndex >= bf.piecesCount {
		return fmt.Errorf("pieceIndex is out of range [0, %d)", bf.piecesCount)
	}
	bitIndex := pieceIndex % bits
	bf.bitfield[byteIndex] |= 1 << (7 - bitIndex)
	return nil
}

func (bf *Bitfield) Has(pieceIndex int) bool {
	bf.lock.RLock()
	defer bf.lock.RUnlock()
	byteIndex := pieceIndex / bits
	bitIndex := pieceIndex % bits
	if pieceIndex > bf.piecesCount-1 {
		panic("pieceIndex out of range")
	}
	byteValue := bf.bitfield[byteIndex]
	mask := byte(1 << (7 - bitIndex))
	return (byteValue & mask) != 0
}

func (bf *Bitfield) initCompletedBitfield() {
	hasSpareBytes := bf.piecesCount%bits != 0
	for i := range bf.bitfield {
		val := byte(0xFF)
		isLastByte := i == len(bf.bitfield)-1
		if hasSpareBytes && isLastByte {
			lastBytePiecesCount := bf.piecesCount % bits
			val = val>>lastBytePiecesCount ^ val
		}
		bf.completedBitfield[i] = val
	}
}

func (*Bitfield) countSetBits(b byte) int {
	count := 0
	for b != 0 {
		count += int(b & 1)
		b >>= 1
	}
	return count
}

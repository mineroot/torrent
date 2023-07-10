package bitfield

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestNew(t *testing.T) {
	bf := New(7)
	assert.Equal(t, bf.BitfieldSize(), 1)
	bf = New(8)
	assert.Equal(t, bf.BitfieldSize(), 1)
	bf = New(9)
	assert.Equal(t, bf.BitfieldSize(), 2)
}

func TestBitfield_IsCompleted(t *testing.T) {
	piecesCount := 71
	bf := New(piecesCount)
	assert.False(t, bf.IsCompleted())
	bf.SetPiece(1)
	assert.False(t, bf.IsCompleted())
	for i := 0; i < piecesCount; i++ {
		bf.SetPiece(i)
	}
	assert.True(t, bf.IsCompleted())

	piecesCount = 8
	bf = New(piecesCount)
	for i := 0; i < piecesCount; i++ {
		bf.SetPiece(i)
	}
	assert.True(t, bf.IsCompleted())
}

func TestBitfield_SetPiece(t *testing.T) {
	piecesCount := 7
	bf := New(piecesCount)
	bf.SetPiece(0)
	assert.Equal(t, byte(0b10000000), bf.bitfield[0])
	bf.SetPiece(2)
	assert.Equal(t, byte(0b10100000), bf.bitfield[0])
	bf.SetPiece(6)
	assert.Equal(t, byte(0b10100010), bf.bitfield[0])
	assert.Panics(t, func() {
		bf.SetPiece(7)
	})
}

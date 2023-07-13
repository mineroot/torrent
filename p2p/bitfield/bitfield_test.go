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

func TestFromPayload(t *testing.T) {
	tests := map[string]struct {
		payload     []byte
		piecesCount int
		err         error
	}{
		"ok without spare bits": {
			payload:     []byte{0xFF, 0xFF, 0xFF},
			piecesCount: 24,
			err:         nil,
		},
		"ok with spare bits": {
			payload:     []byte{0xFF, 0xFF, 0b11111000},
			piecesCount: 21,
			err:         nil,
		},
		"err without spare bits": {
			payload:     []byte{0xFF, 0xFF, 0xFF},
			piecesCount: 32,
			err:         ErrMalformedBitfield,
		},
		"err with spare bits": {
			payload:     []byte{0xFF, 0xFF, 0b11111001}, // three last bits should be zero
			piecesCount: 21,
			err:         ErrMalformedBitfield,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf, err := FromPayload(test.payload, test.piecesCount)
			if test.err != nil {
				assert.ErrorIs(t, err, test.err)
				assert.Nil(t, bf)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.payload, bf.bitfield)
				assert.Equal(t, test.piecesCount, bf.piecesCount)
			}
		})
	}
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

func TestBitfield_Has(t *testing.T) {
	piecesCount := 7
	bf := New(piecesCount)
	for i := 0; i < piecesCount; i++ {
		assert.False(t, bf.Has(i))
	}

	piecesCount = 21
	bf, err := FromPayload([]byte{0xFF, 0xFF, 0b11111000}, piecesCount)
	assert.NoError(t, err)
	for i := 0; i < piecesCount; i++ {
		assert.True(t, bf.Has(i))
	}
	assert.PanicsWithValue(t, "pieceIndex out of range", func() {
		bf.Has(piecesCount + 1)
	})
}

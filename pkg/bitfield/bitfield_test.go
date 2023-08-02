package bitfield

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	err := bf.Set(1)
	assert.NoError(t, err)
	assert.False(t, bf.IsCompleted())
	for i := 0; i < piecesCount; i++ {
		err = bf.Set(i)
		assert.NoError(t, err)
	}
	assert.True(t, bf.IsCompleted())

	piecesCount = 8
	bf = New(piecesCount)
	for i := 0; i < piecesCount; i++ {
		err = bf.Set(i)
		assert.NoError(t, err)
	}
	assert.True(t, bf.IsCompleted())
}

func TestBitfield_SetPiece(t *testing.T) {
	piecesCount := 7
	bf := New(piecesCount)
	err := bf.Set(0)
	assert.NoError(t, err)
	assert.Equal(t, byte(0b10000000), bf.bitfield[0])
	err = bf.Set(2)
	assert.NoError(t, err)
	assert.Equal(t, byte(0b10100000), bf.bitfield[0])
	err = bf.Set(6)
	assert.NoError(t, err)
	assert.Equal(t, byte(0b10100010), bf.bitfield[0])

	err = bf.Set(7)
	assert.Error(t, err)
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

func TestBitfield_DownloadedPiecesCount(t *testing.T) {
	tests := map[string]struct {
		payload          []byte
		piecesCount      int
		downloadedPieces int
	}{
		"16": {
			payload:          []byte{0x0, 0xFF, 0xFF},
			piecesCount:      24,
			downloadedPieces: 16,
		},
		"21": {
			payload:          []byte{0xFF, 0xFF, 0b11111000},
			piecesCount:      21,
			downloadedPieces: 21,
		},
		"13": {
			payload:          []byte{0b10101010, 0b10101010, 0b11111000},
			piecesCount:      21,
			downloadedPieces: 13,
		},
		"0": {
			payload:          []byte{0x0, 0x0, 0x0},
			piecesCount:      24,
			downloadedPieces: 0,
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bf, err := FromPayload(test.payload, test.piecesCount)
			require.NoError(t, err)
			assert.Equal(t, test.downloadedPieces, bf.DownloadedPiecesCount())
		})
	}
}

func TestBitfield_Interested(t *testing.T) {
	type test struct {
		name         string
		bf, bf2      *Bitfield
		isInterested bool
		panics       bool
	}
	hasFirstPiece := New(10)
	_ = hasFirstPiece.Set(0)

	hasLastPiece := New(10)
	_ = hasLastPiece.Set(9)

	hasFirstAndLastPiece := New(10)
	_ = hasFirstAndLastPiece.Set(0)
	_ = hasFirstAndLastPiece.Set(9)

	hasAllPieces, err := FromPayload([]byte{0xFF, 0b11000000}, 10)
	require.NoError(t, err)

	tests := []test{
		{
			name:         "piecesCount doesn't match",
			bf:           New(10),
			bf2:          New(9),
			isInterested: false,
			panics:       true,
		},
		{
			name:         "empty & empty",
			bf:           New(10),
			bf2:          New(10),
			isInterested: false,
		},
		{
			name:         "hasLastPiece & empty",
			bf:           hasLastPiece,
			bf2:          New(10),
			isInterested: false,
		},
		{
			name:         "hasLastPiece & hasLastPiece",
			bf:           hasLastPiece,
			bf2:          hasLastPiece,
			isInterested: false,
		},
		{
			name:         "hasFirstAndLastPiece & hasLastPiece",
			bf:           hasFirstAndLastPiece,
			bf2:          hasLastPiece,
			isInterested: false,
		},
		{
			name:         "hasAllPieces & hasFirstAndLastPiece",
			bf:           hasAllPieces,
			bf2:          hasFirstAndLastPiece,
			isInterested: false,
		},
		{
			name:         "empty & hasLastPiece",
			bf:           New(10),
			bf2:          hasLastPiece,
			isInterested: true,
		},
		{
			name:         "hasFirstPiece & hasLastPiece",
			bf:           hasFirstPiece,
			bf2:          hasLastPiece,
			isInterested: true,
		},
		{
			name:         "hasLastPiece & hasFirstPiece",
			bf:           hasLastPiece,
			bf2:          hasFirstPiece,
			isInterested: true,
		},
		{
			name:         "hasLastPiece & hasFirstAndLastPiece",
			bf:           hasLastPiece,
			bf2:          hasFirstAndLastPiece,
			isInterested: true,
		},
		{
			name:         "empty & hasFirstAndLastPiece",
			bf:           New(10),
			bf2:          hasFirstAndLastPiece,
			isInterested: true,
		},
		{
			name:         "hasFirstAndLastPiece & hasAllPieces",
			bf:           hasFirstAndLastPiece,
			bf2:          hasAllPieces,
			isInterested: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := func() {
				assert.Equal(t, tt.isInterested, tt.bf.Interested(tt.bf2))
			}
			if tt.panics {
				require.Panics(t, f)
			} else {
				require.NotPanics(t, f)
			}
		})
	}
}

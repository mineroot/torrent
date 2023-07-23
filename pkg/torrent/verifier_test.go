package torrent

import (
	"bytes"
	"crypto/sha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"testing"
)

func TestVerifyPiece(t *testing.T) {
	const pieceSize = 1 << 18 // 256 KiB
	const piecesCount = 10
	buf := make([]byte, piecesCount*pieceSize)
	random := rand.New(rand.NewSource(0))
	_, err := random.Read(buf)

	require.NoError(t, err)
	hashes := make([]Hash, 0, piecesCount)
	reader := bytes.NewReader(buf)
	for i := 0; i < piecesCount; i++ {
		buf := make([]byte, pieceSize)
		_, err := reader.Read(buf)
		require.NoError(t, err)
		hashes = append(hashes, sha1.Sum(buf))
	}
	reader.Reset(buf)
	for i := 0; i < piecesCount; i++ {
		ok, err := VerifyPiece(reader, hashes[i], i, pieceSize)
		require.NoError(t, err)
		assert.True(t, ok)
	}

	invalidPiece := make([]byte, pieceSize)
	ok, err := VerifyPiece(bytes.NewReader(invalidPiece), hashes[0], 1, pieceSize)
	require.NoError(t, err)
	assert.False(t, ok)
}

package torrent

import (
	"crypto/sha1"
	"fmt"
	"io"
)

func VerifyPiece(r io.ReaderAt, hash Hash, pieceIndex, pieceSize int) (bool, error) {
	offset := int64(pieceIndex * pieceSize)
	buf := make([]byte, pieceSize)
	n, err := r.ReadAt(buf, offset)
	if err != nil && err != io.EOF {
		return false, fmt.Errorf("unable to read at %d: %w", offset, err)
	}
	if n != pieceSize {
		buf = buf[:n]
	}
	return sha1.Sum(buf) == hash, nil
}

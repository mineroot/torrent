package p2p

import (
	"crypto/sha1"
	"fmt"
	"golang.org/x/sync/errgroup"
	"io"
	"torrent/bitfield"
)

type BitfieldCalculator interface {
	Calculate(r io.ReaderAt, pieceLen int, hashes []Hash) (*bitfield.Bitfield, error)
}

type bitfieldConcurrentCalculator struct{}

func (bitfieldConcurrentCalculator) Calculate(r io.ReaderAt, pieceLen int, hashes []Hash) (*bitfield.Bitfield, error) {
	const bits = 8
	piecesCount := len(hashes)
	bf := bitfield.New(piecesCount)
	if r == nil {
		// return bitfield with all zeroes
		return bf, nil
	}
	g := new(errgroup.Group)
	for i := 0; i < bf.BitfieldSize(); i++ {
		i := i
		g.Go(func() error {
			for j := 0; j < bits; j++ {
				pieceIndex := i*bits + j
				offset := int64(pieceIndex * pieceLen)
				buf := make([]byte, pieceLen)
				n, err := r.ReadAt(buf, offset)
				if err != nil && err != io.EOF {
					return fmt.Errorf("unable to read at %d: %w", offset, err)
				}
				if n != pieceLen {
					buf = buf[:n]
				}
				if sha1.Sum(buf) == hashes[pieceIndex] {
					bf.SetPiece(pieceIndex)
				}
			}
			return nil
		})
	}
	err := g.Wait()
	return bf, err
}

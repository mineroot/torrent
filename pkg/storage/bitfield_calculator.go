package storage

import (
	"golang.org/x/sync/errgroup"
	"io"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/torrent"
)

type BitfieldCalculator interface {
	Calculate(r io.ReaderAt, hashes []torrent.Hash, pieceLen int) (*bitfield.Bitfield, error)
}

type bitfieldConcurrentCalculator struct{}

func (bitfieldConcurrentCalculator) Calculate(r io.ReaderAt, hashes []torrent.Hash, pieceLen int) (*bitfield.Bitfield, error) {
	const bits = 8
	piecesCount := len(hashes)
	bf := bitfield.New(piecesCount)
	g := new(errgroup.Group)
	for i := 0; i < bf.BitfieldSize(); i++ {
		i := i
		g.Go(func() error {
			for j := 0; j < bits; j++ {
				pieceIndex := i*bits + j
				if pieceIndex >= len(hashes) {
					return nil
				}
				ok, err := torrent.VerifyPiece(r, hashes[pieceIndex], pieceIndex, pieceLen)
				if err != nil {
					return err
				}
				if ok {
					bf.Set(pieceIndex)
				}
			}
			return nil
		})
	}
	err := g.Wait()
	return bf, err
}

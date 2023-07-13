package storage

import (
	"golang.org/x/sync/errgroup"
	"io"
	"torrent/p2p/bitfield"
	"torrent/p2p/torrent"
)

type BitfieldCalculator interface {
	Calculate(r io.ReaderAt, hashes []torrent.Hash, pieceLen int) (*bitfield.Bitfield, error)
}

type bitfieldConcurrentCalculator struct{}

func (bitfieldConcurrentCalculator) Calculate(r io.ReaderAt, hashes []torrent.Hash, pieceLen int) (*bitfield.Bitfield, error) {
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
				ok, err := torrent.VerifyPiece(r, hashes[pieceIndex], pieceIndex, pieceLen)
				if err != nil {
					return err
				}
				if ok {
					bf.SetPiece(pieceIndex)
				}
			}
			return nil
		})
	}
	err := g.Wait()
	return bf, err
}

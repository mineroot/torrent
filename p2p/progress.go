package p2p

import "torrent/p2p/torrent"

type ProgressID int

const (
	ProgressPieceDownloaded ProgressID = iota
	ProgressConnRead
)

type Progress interface {
	ID() ProgressID
	Hash() torrent.Hash
	Value() any
}

type progress struct {
	id    ProgressID
	hash  torrent.Hash
	value int
}

func (p *progress) ID() ProgressID {
	return p.id
}

func (p *progress) Hash() torrent.Hash {
	return p.hash
}

func (p *progress) Value() any {
	return p.value
}

func NewPieceDownloaded(hash torrent.Hash, pieceIndex int) Progress {
	return &progress{
		id:    ProgressPieceDownloaded,
		hash:  hash,
		value: pieceIndex,
	}
}

func NewConnRead(hash torrent.Hash, length int) Progress {
	return &progress{
		id:    ProgressConnRead,
		hash:  hash,
		value: length,
	}
}

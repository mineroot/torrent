package p2p

import "torrent/p2p/torrent"

type Progress struct {
	hash       torrent.Hash
	pieceIndex int
}

func (p *Progress) Hash() torrent.Hash {
	return p.hash
}

func (p *Progress) PieceIndex() int {
	return p.pieceIndex
}

func NewProgress(hash torrent.Hash, pieceIndex int) *Progress {
	return &Progress{hash: hash, pieceIndex: pieceIndex}
}

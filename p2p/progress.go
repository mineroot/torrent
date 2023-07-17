package p2p

import "torrent/p2p/torrent"

type ProgressPieceDownloaded struct {
	Hash torrent.Hash
}

func NewProgressPieceDownloaded(hash torrent.Hash) *ProgressPieceDownloaded {
	return &ProgressPieceDownloaded{
		Hash: hash,
	}
}

type ProgressConnRead struct {
	Hash  torrent.Hash
	Bytes int
}

func NewProgressConnRead(hash torrent.Hash, bytesRead int) *ProgressConnRead {
	return &ProgressConnRead{
		Hash:  hash,
		Bytes: bytesRead,
	}
}

type ProgressSpeed struct {
	Hash  torrent.Hash
	Speed int
}

func NewProgressSpeed(hash torrent.Hash, speed int) *ProgressSpeed {
	return &ProgressSpeed{
		Hash:  hash,
		Speed: speed,
	}
}

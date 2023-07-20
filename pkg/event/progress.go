package event

import "github.com/mineroot/torrent/pkg/torrent"

type ProgressPieceDownloaded struct {
	Hash            torrent.Hash
	DownloadedCount int
}

func NewProgressPieceDownloaded(hash torrent.Hash, count int) *ProgressPieceDownloaded {
	return &ProgressPieceDownloaded{
		Hash:            hash,
		DownloadedCount: count,
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

package p2p

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"torrent/bitfield"
)

type StorageReader interface {
	Iterator() <-chan *TorrentFile
	Get(infoHash Hash) *TorrentFile
	GetBitfield(infoHash Hash) *bitfield.Bitfield
}

type Storage struct {
	calculator BitfieldCalculator
	lock       sync.RWMutex
	torrents   map[Hash]*TorrentFile
	bitfields  map[Hash]*bitfield.Bitfield
}

func NewStorage() *Storage {
	return &Storage{
		calculator: &bitfieldConcurrentCalculator{},
		torrents:   make(map[Hash]*TorrentFile),
		bitfields:  make(map[Hash]*bitfield.Bitfield),
	}
}

func (s *Storage) Iterator() <-chan *TorrentFile {
	s.lock.RLock()
	ch := make(chan *TorrentFile, len(s.torrents))
	go func() {
		defer func() {
			s.lock.RUnlock()
			close(ch)
		}()
		for _, torrent := range s.torrents {
			ch <- torrent
		}
	}()
	return ch
}

func (s *Storage) Get(infoHash Hash) *TorrentFile {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.torrents[infoHash]
}

func (s *Storage) GetBitfield(infoHash Hash) *bitfield.Bitfield {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.bitfields[infoHash]
}

func (s *Storage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.torrents)
}

func (s *Storage) Set(infoHash Hash, torrent *TorrentFile) error {
	if torrent == nil {
		panic("torrent must not be nil")
	}
	if infoHash != torrent.InfoHash {
		panic("infoHash is not equal torrent.InfoHash")
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.torrents[infoHash] = torrent

	path := filepath.Join(torrent.DownloadDir, torrent.Name)
	file, err := os.Open(path)
	var r io.ReaderAt
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("unable not open file %s: %w", path, err)
		}
	} else {
		r = file
	}
	if file != nil {
		defer file.Close()
	}

	bf, err := s.calculator.Calculate(r, torrent.PieceLength, torrent.PieceHashes)
	if err != nil {
		return fmt.Errorf("unable to calculate bitfield: %w", err)
	}
	s.bitfields[infoHash] = bf
	return nil
}

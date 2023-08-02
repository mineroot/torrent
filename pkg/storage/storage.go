package storage

import (
	"fmt"
	"github.com/spf13/afero"
	"sync"

	"github.com/mineroot/torrent/pkg/torrent"
)

type Reader interface {
	Get(infoHash torrent.Hash) *TorrentData
	Iterator() <-chan *TorrentData
}

type Storage struct {
	lock sync.RWMutex
	tds  map[torrent.Hash]*TorrentData

	fs afero.Fs
}

func NewStorage(fs afero.Fs) *Storage {
	return &Storage{
		fs:  fs,
		tds: make(map[torrent.Hash]*TorrentData),
	}
}

func (s *Storage) Iterator() <-chan *TorrentData {
	s.lock.RLock()
	ch := make(chan *TorrentData, len(s.tds))
	go func() {
		defer func() {
			s.lock.RUnlock()
			close(ch)
		}()
		for _, td := range s.tds {
			ch <- td
		}
	}()
	return ch
}

func (s *Storage) Get(infoHash torrent.Hash) *TorrentData {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.tds[infoHash]
}

func (s *Storage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.tds)
}

func (s *Storage) Set(torrent *torrent.File) error {
	if torrent == nil {
		panic("torrent must not be nil")
	}

	td, err := NewTorrentData(s.fs, torrent)
	if err != nil {
		return fmt.Errorf("storage: unable to create torrent data: %w", err)
	}
	s.lock.Lock()
	defer s.lock.Unlock()
	s.tds[torrent.InfoHash] = td
	return nil
}

func (s *Storage) Close() (lastErr error) {
	s.lock.Lock()
	defer s.lock.Unlock()
	for _, td := range s.tds {
		if err := td.Close(); err != nil {
			lastErr = err
		}
	}
	return
}

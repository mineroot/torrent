package p2p

import "sync"

type StorageGetter interface {
	Get(hash Hash) *TorrentFile
}

type Storage struct {
	lock     sync.RWMutex
	torrents map[Hash]*TorrentFile
}

func NewStorage() *Storage {
	return &Storage{
		torrents: make(map[Hash]*TorrentFile),
	}
}

func (s *Storage) IterateTorrents() <-chan *TorrentFile {
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

func (s *Storage) Get(hash Hash) *TorrentFile {
	s.lock.RLock()
	defer s.lock.RUnlock()
	if torrent, ok := s.torrents[hash]; ok {
		return torrent
	}
	return nil
}

func (s *Storage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.torrents)
}

func (s *Storage) Set(hash Hash, torrent *TorrentFile) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.torrents[hash] = torrent
}

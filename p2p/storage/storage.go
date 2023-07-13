package storage

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"torrent/p2p/bitfield"
	"torrent/p2p/torrent"
)

type Reader interface {
	Iterator() <-chan *torrent.File
	Get(infoHash torrent.Hash) *torrent.File
	GetBitfield(infoHash torrent.Hash) *bitfield.Bitfield
	GetFile(infoHash torrent.Hash) *os.File
}

type Storage struct {
	calculator BitfieldCalculator

	lock      sync.RWMutex
	torrents  map[torrent.Hash]*torrent.File
	bitfields map[torrent.Hash]*bitfield.Bitfield
	files     map[torrent.Hash]*os.File
}

func NewStorage() *Storage {
	return &Storage{
		calculator: &bitfieldConcurrentCalculator{},
		torrents:   make(map[torrent.Hash]*torrent.File),
		bitfields:  make(map[torrent.Hash]*bitfield.Bitfield),
		files:      make(map[torrent.Hash]*os.File),
	}
}

func (s *Storage) Iterator() <-chan *torrent.File {
	s.lock.RLock()
	ch := make(chan *torrent.File, len(s.torrents))
	go func() {
		defer func() {
			s.lock.RUnlock()
			close(ch)
		}()
		for _, t := range s.torrents {
			ch <- t
		}
	}()
	return ch
}

func (s *Storage) Get(infoHash torrent.Hash) *torrent.File {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.torrents[infoHash]
}

func (s *Storage) GetBitfield(infoHash torrent.Hash) *bitfield.Bitfield {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.bitfields[infoHash]
}

func (s *Storage) GetFile(infoHash torrent.Hash) *os.File {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.files[infoHash]
}

func (s *Storage) Len() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return len(s.torrents)
}

func (s *Storage) Set(infoHash torrent.Hash, torrent *torrent.File) error {
	if torrent == nil {
		panic("torrent must not be nil")
	}
	if infoHash != torrent.InfoHash {
		panic("infoHash is not equal torrent.InfoHash")
	}
	s.lock.Lock()
	defer s.lock.Unlock()

	// open or crate file for read and write
	path := filepath.Join(torrent.DownloadDir, torrent.Name)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return fmt.Errorf("unable not open file %s: %w", path, err)
	}

	// calculate bitfield
	bf, err := s.calculator.Calculate(file, torrent.PieceHashes, torrent.PieceLength)
	if err != nil {
		return fmt.Errorf("unable to calculate bitfield: %w", err)
	}

	// fill file with zeros if just created
	fInfo, err := file.Stat()
	if err != nil {
		return fmt.Errorf("unable to get file stat")
	}
	if fInfo.Size() == 0 {
		_, err = file.Write(make([]byte, torrent.Length))
		if err != nil {
			return fmt.Errorf("unable to fill file with zeros")
		}
	}
	// add to storage all torrent's bitfield, file handler and torrent itself
	s.torrents[infoHash] = torrent
	s.bitfields[infoHash] = bf
	s.files[infoHash] = file

	return nil
}

func (s *Storage) Close() error {
	s.lock.RLock()
	defer s.lock.RUnlock()
	var err error
	for _, file := range s.files {
		err = file.Close()
	}
	return err
}

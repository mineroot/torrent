package p2p

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"torrent/bitfield"
)

type StorageReader interface {
	Iterator() <-chan *TorrentFile
	Get(infoHash Hash) *TorrentFile
	GetBitfield(infoHash Hash) *bitfield.Bitfield
	GetFile(infoHash Hash) *os.File
}

type Storage struct {
	calculator BitfieldCalculator

	lock      sync.RWMutex
	torrents  map[Hash]*TorrentFile
	bitfields map[Hash]*bitfield.Bitfield
	files     map[Hash]*os.File
}

func NewStorage() *Storage {
	return &Storage{
		calculator: &bitfieldConcurrentCalculator{},
		torrents:   make(map[Hash]*TorrentFile),
		bitfields:  make(map[Hash]*bitfield.Bitfield),
		files:      make(map[Hash]*os.File),
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

func (s *Storage) GetFile(infoHash Hash) *os.File {
	s.lock.RLock()
	defer s.lock.RUnlock()
	return s.files[infoHash]
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

	// open or crate file for read and write
	path := filepath.Join(torrent.DownloadDir, torrent.Name)
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0664)
	if err != nil {
		return fmt.Errorf("unable not open file %s: %w", path, err)
	}

	// calculate bitfield
	bf, err := s.calculator.Calculate(file, torrent.PieceLength, torrent.PieceHashes)
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

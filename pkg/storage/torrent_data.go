package storage

import (
	"errors"
	"fmt"
	"github.com/spf13/afero"
	"golang.org/x/sync/errgroup"
	"io"
	"os"
	"path/filepath"
	"sync"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/torrent"
)

type TorrentData struct {
	lock sync.Mutex
	fds  []afero.File

	fs       afero.Fs
	torrent  *torrent.File
	bitfield *bitfield.Bitfield
	sizes    []int64
}

func NewTorrentData(fs afero.Fs, torrent *torrent.File) (*TorrentData, error) {
	td := &TorrentData{
		fs:      fs,
		torrent: torrent,
		fds:     make([]afero.File, torrent.DownloadFilesCount()),
		sizes:   make([]int64, torrent.DownloadFilesCount()),
	}
	var totalSize int64
	for i, file := range torrent.DownloadFiles {
		dirPath := filepath.Dir(file.Name)
		if err := fs.MkdirAll(dirPath, 0755); err != nil {
			return nil, fmt.Errorf("storage: unable to create directory(es): %w", err)
		}
		fd, err := td.fs.OpenFile(file.Name, os.O_RDWR|os.O_CREATE, 0664)
		if err != nil {
			return nil, fmt.Errorf("storage: unable to open file: %w", err)
		}
		fInfo, err := fd.Stat()
		if err != nil {
			return nil, fmt.Errorf("storage: unable to get file stat: %w", err)
		}
		if fInfo.Size() != file.Length {
			// create sparse file
			err = fd.Truncate(file.Length)
			if err != nil {
				return nil, fmt.Errorf("storage: unable to truncate: %w", err)
			}
		}
		td.fds[i] = fd
		td.sizes[i] = file.Length
		totalSize += file.Length
	}
	if totalSize != td.torrent.TotalLength() {
		return nil, fmt.Errorf("storage: unable to create torrent data")
	}

	err := td.calcBitfield()
	if err != nil {
		return nil, fmt.Errorf("unable to calculate bitfield: %w", err)
	}

	return td, nil
}

func (td *TorrentData) ReadAt(p []byte, off int64) (n int, err error) {
	fdIndex, off, err := td.calcFdIndexAndOffset(off)
	if err != nil {
		return 0, io.EOF
	}

	td.lock.Lock()
	defer td.lock.Unlock()
	var nn int
	for nn < len(p) {
		n, err = td.fds[fdIndex].ReadAt(p[nn:], off)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nn, err
		}
		nn += n
		fdIndex++
		off = 0
		if fdIndex >= len(td.fds) {
			break
		}
	}
	if nn < len(p) {
		err = io.EOF
	}
	return nn, err
}

func (td *TorrentData) WriteAt(p []byte, off int64) (n int, err error) {
	fdIndex, off, err := td.calcFdIndexAndOffset(off)
	if err != nil {
		return 0, io.ErrShortWrite
	}

	td.lock.Lock()
	defer td.lock.Unlock()
	var nn int
	for nn < len(p) {
		// truncate p if it cannot be fully written to the td.fds[fdIndex]
		// so the rest will be written to the next td.fds[fdIndex]
		limit := int64(nn) + td.sizes[fdIndex] - off
		if limit > int64(len(p)) {
			limit = int64(len(p))
		}
		n, err = td.fds[fdIndex].WriteAt(p[nn:limit], off)
		if err != nil {
			return nn, err
		}
		nn += n
		fdIndex++
		off = 0
		if fdIndex >= len(td.fds) {
			break
		}
	}
	if nn < len(p) {
		err = io.ErrShortWrite
	}
	return nn, err
}

func (td *TorrentData) Torrent() *torrent.File {
	return td.torrent
}

func (td *TorrentData) InfoHash() torrent.Hash {
	return td.torrent.InfoHash
}

func (td *TorrentData) Bitfield() *bitfield.Bitfield {
	return td.bitfield
}

func (td *TorrentData) Close() (lastErr error) {
	for _, fd := range td.fds {
		lastErr = fd.Close()
	}
	return lastErr
}

func (td *TorrentData) calcBitfield() error {
	const bits = 8
	piecesCount := len(td.torrent.PieceHashes)
	td.bitfield = bitfield.New(piecesCount)
	g := new(errgroup.Group)
	for i := 0; i < td.bitfield.BitfieldSize(); i++ {
		i := i
		g.Go(func() error {
			for j := 0; j < bits; j++ {
				pieceIndex := i*bits + j
				if pieceIndex >= len(td.torrent.PieceHashes) {
					return nil
				}
				ok, err := torrent.VerifyPiece(td, td.torrent.PieceHashes[pieceIndex], pieceIndex, td.torrent.PieceLength)
				if err != nil {
					return err
				}
				if ok {
					if err = td.bitfield.Set(pieceIndex); err != nil {
						panic(fmt.Errorf("storage: %w", err))
					}
				}
			}
			return nil
		})
	}
	return g.Wait()
}

func (td *TorrentData) calcFdIndexAndOffset(off int64) (int, int64, error) {
	var fdIndex int
	if off < 0 || off >= td.torrent.TotalLength() {
		return fdIndex, off, fmt.Errorf("offset out of bounds")
	}
	var sizes int64
	for i := 0; i < len(td.sizes); i++ {
		sizes += td.sizes[i]
		if off < sizes {
			off = off - (sizes - td.sizes[i])
			fdIndex = i
			break
		}
	}
	return fdIndex, off, nil
}

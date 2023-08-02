package storage

import (
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"math/rand"
	"os"
	"testing"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/torrent"
)

const downloadDir = "~/Downloads"
const downloadFilePath = downloadDir + "/cat.png"

func TestStorage(t *testing.T) {
	t.Run("panic (nil file)", func(t *testing.T) {
		s, _, _ := setUp(t)
		assert.Panics(t, func() {
			_ = s.Set(nil)
		})
	})
	t.Run("downloaded file not creates + additional checks", func(t *testing.T) {
		s, memFs, torr := setUp(t)
		// assert downloaded file doesn't exist yet
		exists, err := afero.Exists(memFs, downloadFilePath)
		require.NoError(t, err)
		assert.False(t, exists)
		// add torrent to storage
		assert.NotPanics(t, func() {
			err := s.Set(torr)
			assert.Equal(t, 1, s.Len())
			require.NoError(t, err)
		})
		// assert downloaded file does exist
		exists, err = afero.Exists(memFs, downloadFilePath)
		require.NoError(t, err)
		assert.True(t, exists)
		// asser downloaded file filled with zeroes
		fileBytes, err := afero.ReadFile(memFs, downloadFilePath)
		require.NoError(t, err)
		assert.Equal(t, len(fileBytes), int(torr.TotalLength()))
		assert.Equal(t, fileBytes, make([]byte, torr.TotalLength()))
		// assert  getter
		assert.Equal(t, torr, s.Get(torr.InfoHash).Torrent())
		// assert file descriptor getter
		fd, err := memFs.Open(downloadFilePath)
		require.NoError(t, err)
		assert.Equal(t, fd.Name(), s.Get(torr.InfoHash).fds[0].Name())
		// assert bitfield
		bf := s.Get(torr.InfoHash).Bitfield()
		assert.False(t, bf.IsCompleted())
		assert.Zero(t, bf.DownloadedPiecesCount())
	})
	t.Run("file partially downloaded", func(t *testing.T) {
		s, memFs, tFile := setUp(t)
		bf := createPartiallyDownloadedFile(t, memFs, tFile)
		assert.NotPanics(t, func() {
			err := s.Set(tFile)
			assert.Equal(t, 1, s.Len())
			require.NoError(t, err)
		})
		actualBf := s.Get(tFile.InfoHash).Bitfield()
		assert.Equal(t, bf, actualBf)
		assert.False(t, actualBf.IsCompleted())
	})
	t.Run("file fully downloaded", func(t *testing.T) {
		s, memFs, tFile := setUp(t)
		bf := createFullyDownloadedFile(t, memFs, tFile)
		assert.NotPanics(t, func() {
			err := s.Set(tFile)
			assert.Equal(t, 1, s.Len())
			require.NoError(t, err)
		})
		actualBf := s.Get(tFile.InfoHash).Bitfield()
		assert.Equal(t, bf, actualBf)
		assert.True(t, actualBf.IsCompleted())
	})
}

func setUp(t *testing.T) (*Storage, afero.Fs, *torrent.File) {
	memFs := afero.NewMemMapFs()
	s := NewStorage(memFs)
	tFile, err := torrent.Open("../../testdata/cat.png.torrent", downloadDir)
	require.NoError(t, err)
	return s, memFs, tFile
}

func createPartiallyDownloadedFile(t *testing.T, fs afero.Fs, tFile *torrent.File) *bitfield.Bitfield {
	originalFile, err := os.ReadFile("../../testdata/downloads/remote/cat.png")
	require.NoError(t, err)
	downloadedFileD, err := fs.OpenFile(downloadFilePath, os.O_WRONLY|os.O_CREATE, 0664)
	defer downloadedFileD.Close()
	require.NoError(t, err)
	random := rand.New(rand.NewSource(0))
	bf := bitfield.New(tFile.PiecesCount())
	for i := 0; i < tFile.PiecesCount(); i++ {
		toCopy := random.Intn(2) == 0 // copy ~50% pieces
		// we need to copy the last piece anyway because downloadedFileD size may not be the same as originalFile
		isLastPiece := i == tFile.PiecesCount()-1
		if toCopy || isLastPiece {
			err = bf.Set(i)
			assert.NoError(t, err)
			from := i * tFile.PieceLength
			to := from + tFile.PieceLength
			if isLastPiece {
				to = len(originalFile)
			}
			pieceBytes := originalFile[from:to]
			_, err = downloadedFileD.WriteAt(pieceBytes, int64(from))
			require.NoError(t, err)
		}
	}
	downloadedFileStat, err := downloadedFileD.Stat()
	require.NoError(t, err)
	require.Equal(t, int64(len(originalFile)), downloadedFileStat.Size())
	return bf
}

func createFullyDownloadedFile(t *testing.T, fs afero.Fs, tFile *torrent.File) *bitfield.Bitfield {
	originalFile, err := os.ReadFile("../../testdata/downloads/remote/cat.png")
	require.NoError(t, err)
	downloadedFileD, err := fs.OpenFile(downloadFilePath, os.O_WRONLY|os.O_CREATE, 0664)
	defer downloadedFileD.Close()
	require.NoError(t, err)
	_, err = downloadedFileD.Write(originalFile)
	require.NoError(t, err)
	bf := bitfield.New(tFile.PiecesCount())
	for i := 0; i < tFile.PiecesCount(); i++ {
		err = bf.Set(i)
		assert.NoError(t, err)
	}
	return bf
}

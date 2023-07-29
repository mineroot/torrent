package peer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"io"
	"net"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mineroot/torrent/mocks/github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/download"
	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
)

func TestManager_Run(t *testing.T) {
	conn1, conn2 := createCons()

	downloadingPm, fs, progress, piecesCount := createDownloadingManager(t, conn1)
	seedingPm := createSeedingManager(t, conn2)
	assert.True(t, downloadingPm.IsAlive())
	assert.True(t, seedingPm.IsAlive())
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().Level(zerolog.InfoLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		err := seedingPm.Run(ctx)
		assert.Error(t, err)
		assert.False(t, seedingPm.IsAlive())
	}()
	// cancel ctx when downloaded
	go func() {
		downloaded := 0
		for {
			<-progress
			downloaded++
			if downloaded == piecesCount {
				cancel()
				break
			}
		}
	}()
	err := downloadingPm.Run(l.WithContext(ctx))
	assert.Error(t, err)
	assert.False(t, downloadingPm.IsAlive())
	// check downloaded file is correct
	buf, err := afero.ReadFile(fs, "cat.png")
	require.NoError(t, err)
	sum256 := sha256.Sum256(buf)
	assert.Equal(t, "f3c20915cc8dfacb802d4858afd8c7ca974529f6259e9250bc52594d71042c30", hex.EncodeToString(sum256[:]))
}

func createDownloadingManager(t *testing.T, conn net.Conn) (*Manager, afero.Fs, chan *event.ProgressPieceDownloaded, int) {
	torrentFile, err := torrent.Open("../../testdata/cat.png.torrent", "")
	require.NoError(t, err)
	fs := afero.NewMemMapFs()
	s := storage.NewStorage(fs)
	err = s.Set(torrentFile)
	require.NoError(t, err)

	dialer := peer.NewMockContextDialer(t)
	dialer.EXPECT().DialContext(mock.Anything, mock.Anything, mock.Anything).Return(conn, nil)
	dms := download.CreateDownloadManagers(s)
	progressPieces := make(chan *event.ProgressPieceDownloaded, 1)
	return NewManager(
			ID([]byte("-GO0001-randombytes1")),
			dialer,
			s,
			Peer{InfoHash: torrentFile.InfoHash},
			dms,
			make(chan<- *event.ProgressConnRead, 1000000),
			progressPieces,
		), fs,
		progressPieces,
		torrentFile.PiecesCount()
}

func createSeedingManager(t *testing.T, conn net.Conn) *Manager {
	torrentFile, err := torrent.Open("../../testdata/cat.png.torrent", "../../testdata/downloads/remote")
	require.NoError(t, err)
	s := storage.NewStorage(afero.NewOsFs())
	err = s.Set(torrentFile)
	require.NoError(t, err)

	dialer := peer.NewMockContextDialer(t)
	dms := download.CreateDownloadManagers(s)
	return NewManager(
		ID([]byte("-GO0001-randombytes2")),
		dialer,
		s,
		Peer{Conn: conn},
		dms,
		make(chan<- *event.ProgressConnRead, 1000000),
		make(chan<- *event.ProgressPieceDownloaded, 1000000),
	)
}

func createCons() (net.Conn, net.Conn) {
	conn1 := &conn{pipe: make(chan byte, 512), name: "A"}
	conn2 := &conn{pipe: make(chan byte, 512), peer: conn1, name: "B"}
	conn1.peer = conn2
	return conn1, conn2
}

type conn struct {
	name   string
	closed atomic.Bool
	pipe   chan byte
	peer   *conn
}

func (c *conn) Read(b []byte) (n int, err error) {
	read := 0
	for i := 0; i < len(b); i++ {
		bi, ok := <-c.peer.pipe
		if !ok {
			c.closed.Store(true)
			return read, io.EOF
		}
		read++
		b[i] = bi
	}
	return len(b), nil
}

func (c *conn) Write(b []byte) (n int, err error) {
	wrote := 0
	for i := 0; i < len(b); i++ {
		if c.closed.Load() {
			return wrote, net.ErrClosed
		}
		wrote++
		c.pipe <- b[i]
	}
	return len(b), nil
}

func (c *conn) Close() error {
	close(c.pipe)
	return nil
}

func (c *conn) LocalAddr() net.Addr {
	panic("noop")
}

func (c *conn) RemoteAddr() net.Addr {
	panic("noop")
}

func (c *conn) SetDeadline(time.Time) error {
	return nil
}

func (c *conn) SetReadDeadline(time.Time) error {
	return nil
}

func (c *conn) SetWriteDeadline(time.Time) error {
	return nil
}

package pkg

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
	"github.com/mineroot/torrent/pkg/tracker"
)

func TestClient_Run(t *testing.T) {
	_ = os.Remove("../testdata/downloads/local/cat.png")
	// TODO: 25 sec is too much
	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().Level(zerolog.InfoLevel)
	ft := tracker.NewFakeTracker(":8080")

	g, ctx := errgroup.WithContext(ctx)
	// run fake tracker
	g.Go(func() error {
		err := ft.ListenAndServe()
		assert.ErrorIs(t, err, http.ErrServerClosed)
		return err
	})
	// shout down fake tracker
	g.Go(func() error {
		<-ctx.Done()
		err := ft.Shutdown(context.Background())
		assert.NoError(t, err)
		return err
	})
	time.Sleep(10 * time.Millisecond) // wait till fake tracker starts up
	// run a remote peer's client for seeding
	g.Go(func() error {
		err := createClient(t, 6881, "../testdata/downloads/remote", "-GO0001-remote_peer0").Run(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		return err
	})
	// run a local client for downloading
	g.Go(func() error {
		time.Sleep(10 * time.Millisecond) // wait till remote peer announces itself to the fake tracker
		ctx := l.WithContext(ctx)         // attach logger for a local client only
		err := createClient(t, 6882, "../testdata/downloads/local", "-GO0001-local_peer00").Run(ctx)
		assert.ErrorIs(t, err, context.DeadlineExceeded)
		return err
	})
	assert.Error(t, g.Wait()) // wait 15 seconds till ctx cancels for downloading completion

	// check downloaded file is correct
	buf, err := os.ReadFile("../testdata/downloads/local/cat.png")
	require.NoError(t, err)
	sum256 := sha256.Sum256(buf)
	assert.Equal(t, "f3c20915cc8dfacb802d4858afd8c7ca974529f6259e9250bc52594d71042c30", hex.EncodeToString(sum256[:]))
}

func createClient(t *testing.T, port uint16, downloadDir, peerId string) *Client {
	torrentFile, err := torrent.Open("../testdata/cat.png.torrent", downloadDir)
	require.NoError(t, err)
	s := storage.NewStorage(afero.NewOsFs())
	err = s.Set(torrentFile)
	require.NoError(t, err)
	return NewClient(peer.IDFromString(peerId), port, s)
}

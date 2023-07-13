package p2p

import (
	"bytes"
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"testing"
	"time"
	"torrent/bencode"
	"torrent/mocks/torrent/p2p"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
)

func TestClient_Run(t *testing.T) {
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().Level(zerolog.InfoLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)
	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		err := createRemoteClient(t).Run(ctx)
		assert.NoError(t, err)
	}()
	time.Sleep(time.Millisecond * 1000) // wait until remote listener starts up
	go func() {
		defer wg.Done()
		err := createLocalClient(t).Run(ctx)
		assert.NoError(t, err)
	}()
	<-exit
	cancel()
	wg.Wait()
}

func createLocalClient(t *testing.T) *Client {
	// ip 127.0.0.1, port 6881
	peers := bencode.NewString(string([]byte{0x7F, 0x0, 0x0, 0x1, 0x1A, 0xE1}))
	response := bencode.NewDictionary(
		map[bencode.String]bencode.BenType{
			*bencode.NewString("interval"): bencode.NewInteger(900),
			*bencode.NewString("peers"):    peers,
		},
	)
	buf := &bytes.Buffer{}
	err := bencode.Encode(buf, []bencode.BenType{response})
	require.NoError(t, err)

	announcer := p2p.NewMockAnnouncer(t)
	readCloser := &body{Buffer: buf}
	announcer.EXPECT().Do(mock.Anything).Return(&http.Response{Body: readCloser}, nil)
	torrentFile, err := torrent.Open("../testdata/shiza.png.torrent", "../testdata/downloads/local")
	require.NoError(t, err)
	s := storage.NewStorage()
	err = s.Set(torrentFile.InfoHash, torrentFile)
	require.NoError(t, err)
	return NewClient(PeerID([]byte("-GO0001-local_peer00")), 6882, s, announcer)
}

func createRemoteClient(t *testing.T) *Client {
	peers := bencode.NewString("")
	response := bencode.NewDictionary(
		map[bencode.String]bencode.BenType{
			*bencode.NewString("interval"): bencode.NewInteger(900),
			*bencode.NewString("peers"):    peers,
		},
	)
	buf := &bytes.Buffer{}
	err := bencode.Encode(buf, []bencode.BenType{response})
	require.NoError(t, err)

	announcer := p2p.NewMockAnnouncer(t)
	readCloser := &body{Buffer: buf}
	announcer.EXPECT().Do(mock.Anything).Return(&http.Response{Body: readCloser}, nil)
	torrentFile, err := torrent.Open("../testdata/shiza.png.torrent", "../testdata/downloads/remote")
	require.NoError(t, err)
	s := storage.NewStorage()
	err = s.Set(torrentFile.InfoHash, torrentFile)
	require.NoError(t, err)
	return NewClient(PeerID([]byte("-GO0001-remote_peer0")), 6881, s, announcer)
}

type body struct {
	*bytes.Buffer
}

func (b *body) Close() error {
	return nil
}

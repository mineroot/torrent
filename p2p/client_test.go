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
	"testing"
	"time"
	"torrent/bencode"
	"torrent/mocks/torrent/p2p"
)

func TestClient_Run(t *testing.T) {
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().Level(zerolog.InfoLevel)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)

	go func() {
		err := createRemoteClient(t).Run(ctx)
		assert.NoError(t, err)
	}()
	time.Sleep(time.Millisecond * 1000) // wait until remote listener starts up
	go func() {
		err := createLocalClient(t).Run(ctx)
		assert.NoError(t, err)
	}()

	select {}
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
	torrent, err := Open("../testdata/shiza.png.torrent", "../testdata/downloads/local")
	require.NoError(t, err)
	storage := NewStorage()
	err = storage.Set(torrent.InfoHash, torrent)
	require.NoError(t, err)
	return NewClient(PeerID([]byte("-GO0001-local_peer00")), 6882, storage, announcer)
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
	torrent, err := Open("../testdata/shiza.png.torrent", "../testdata/downloads/remote")
	require.NoError(t, err)
	storage := NewStorage()
	err = storage.Set(torrent.InfoHash, torrent)
	require.NoError(t, err)
	return NewClient(PeerID([]byte("-GO0001-remote_peer0")), 6881, storage, announcer)
}

type body struct {
	*bytes.Buffer
}

func (b *body) Close() error {
	return nil
}

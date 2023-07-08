package p2p

import (
	"context"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"net"
	"os"
	"sync"
	"testing"
)

func TestPeerManager_Run(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().WithContext(ctx)

	torrent := createTorrentFile(t)
	storage := NewStorage()
	storage.Set(torrent.InfoHash, torrent)
	peer := Peer{
		IP:       net.IP{127, 0, 0, 1},
		Port:     listenPort,
		InfoHash: &torrent.InfoHash,
	}

	// remote peer
	wg.Add(2)
	remotePeerClient := NewClient(storage)
	listenerStarted := make(chan struct{}, 1)
	go remotePeerClient.listen(ctx, &wg, listenerStarted)
	go remotePeerClient.managePeers(ctx, &wg)

	<-listenerStarted
	pm := NewPeerManager(storage, peer, nil)
	wg.Add(1)
	handshakeOkCh := make(chan bool, 1)
	go pm.Run(ctx, &wg, handshakeOkCh)

	handshakeOk := <-handshakeOkCh
	assert.True(t, handshakeOk)

	cancel()
	wg.Wait()
}

func createTorrentFile(t *testing.T) *TorrentFile {
	file, err := os.Open("../testdata/debian-12.0.0-amd64-netinst.iso.torrent")
	if err != nil {
		require.NoError(t, err)
	}
	defer file.Close()
	torrent, err := Open(file)
	if err != nil {
		require.NoError(t, err)
	}
	return torrent
}

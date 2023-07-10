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
	"time"
)

func TestPeerManager_Run(t *testing.T) {
	var wg sync.WaitGroup
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().WithContext(ctx)

	remoteTorrent, err := Open("../testdata/shiza.png.torrent", "../testdata/downloads")
	//remoteTorrent, err := Open("../testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
	require.NoError(t, err)
	remoteStorage := NewStorage()
	err = remoteStorage.Set(remoteTorrent.InfoHash, remoteTorrent)
	require.NoError(t, err)
	peer := Peer{
		IP:       net.IP{127, 0, 0, 1},
		Port:     listenPort,
		InfoHash: &remoteTorrent.InfoHash,
	}

	// remote peer
	wg.Add(2)
	remotePeerClient := NewClient(PeerID([]byte("-GO0001-remote_peer0")), remoteStorage)
	listenerStarted := make(chan struct{}, 1)
	go remotePeerClient.listen(ctx, &wg, listenerStarted)
	go remotePeerClient.managePeers(ctx, &wg)

	<-listenerStarted

	localTorrent, err := Open("../testdata/shiza.png.torrent", "")
	require.NoError(t, err)
	localStorage := NewStorage()
	err = localStorage.Set(localTorrent.InfoHash, localTorrent)
	require.NoError(t, err)

	pm := NewPeerManager(PeerID([]byte("-GO0001-local_peer00")), localStorage, peer, nil)
	wg.Add(1)
	handshakeOkCh := make(chan bool, 1)
	go pm.Run(ctx, &wg, handshakeOkCh)

	handshakeOk := <-handshakeOkCh
	assert.True(t, handshakeOk)
	time.Sleep(time.Second * 10) // todo
	cancel()
	wg.Wait()
}

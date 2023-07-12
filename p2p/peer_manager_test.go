package p2p

//
//func TestPeerManager_Run(t *testing.T) {
//	var wg sync.WaitGroup
//	ctx, cancel := context.WithCancel(context.Background())
//	defer cancel()
//	ctx = log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().WithContext(ctx)
//
//	remoteTorrent, err := Open("../testdata/shiza.png.torrent", "../testdata/downloads")
//	//remoteTorrent, err := Open("../testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
//	require.NoError(t, err)
//	remoteStorage := NewStorage()
//	err = remoteStorage.Set(remoteTorrent.InfoHash, remoteTorrent)
//	require.NoError(t, err)
//	peer := Peer{
//		IP:       net.IP{127, 0, 0, 1},
//		Port:     1,
//		InfoHash: remoteTorrent.InfoHash,
//	}
//
//	// remote peer
//	wg.Add(2)
//	remotePeerClient := NewClient(PeerID([]byte("-GO0001-remote_peer0")), 0, remoteStorage, nil)
//	listenerStarted := make(chan struct{}, 1)
//	go remotePeerClient.listen(ctx, &wg, listenerStarted)
//	go remotePeerClient.managePeers(ctx, &wg)
//
//	<-listenerStarted
//
//	localTorrent, err := Open("../testdata/shiza.png.torrent", "")
//	require.NoError(t, err)
//	localStorage := NewStorage()
//	err = localStorage.Set(localTorrent.InfoHash, localTorrent)
//	require.NoError(t, err)
//
//	// <create file>
//	filePath := path.Join(localTorrent.DownloadDir, localTorrent.Name)
//	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE, 0664)
//	require.NoError(t, err)
//	fInfo, err := file.Stat()
//	require.NoError(t, err)
//	if fInfo.Size() == 0 {
//		_, err = file.Write(make([]byte, localTorrent.Length))
//		require.NoError(t, err)
//	}
//	// </create file>
//
//	pm := NewPeerManager(PeerID([]byte("-GO0001-local_peer00")), localStorage, peer, nil)
//	pm.file = file
//	wg.Add(1)
//	handshakeOkCh := make(chan bool, 1)
//	go pm.Run(ctx, &wg, handshakeOkCh)
//
//	handshakeOk := <-handshakeOkCh
//	assert.True(t, handshakeOk)
//
//	workQ := make(chan blockWork)
//	go func() {
//		blocks := divide.Divide(localTorrent.Length, []int{localTorrent.PieceLength, BlockLen})
//		for block := range blocks {
//			workQ <- blockWork{
//				pieceIndex: block.ParentIndex,
//				begin:      block.Begin,
//				len:        block.Len,
//			}
//		}
//		close(workQ)
//	}()
//	go pm.startDownloading(workQ)
//
//	time.Sleep(time.Second * 10) // todo
//	cancel()
//	wg.Wait()
//}

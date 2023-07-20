package pkg

//
//func TestClient_Run(t *testing.T) {
//	initialGrowFactor = 40
//	_ = os.Remove("../testdata/downloads/local/cat.png")
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//
//	var wg sync.WaitGroup
//	wg.Add(2)
//	go func() {
//		defer wg.Done()
//		err := createRemoteClient(t).Run(ctx)
//		assert.ErrorIs(t, err, context.DeadlineExceeded)
//	}()
//
//	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger().Level(zerolog.InfoLevel)
//	go func() {
//		// attach logger only for a local client
//		ctx := l.WithContext(ctx)
//		defer wg.Done()
//		err := createLocalClient(t).Run(ctx)
//		assert.ErrorIs(t, err, context.DeadlineExceeded)
//	}()
//	wg.Wait()
//
//	// check downloaded file is correct
//	buf, err := os.ReadFile("../testdata/downloads/local/cat.png")
//	require.NoError(t, err)
//	sum256 := sha256.Sum256(buf)
//	assert.Equal(t, "f3c20915cc8dfacb802d4858afd8c7ca974529f6259e9250bc52594d71042c30", hex.EncodeToString(sum256[:]))
//}
//
//func createLocalClient(t *testing.T) *Client {
//	// ip 127.0.0.1, port 6881
//	peers := bencode.NewString(string([]byte{0x7F, 0x0, 0x0, 0x1, 0x1A, 0xE1}))
//	response := bencode.NewDictionary(
//		map[bencode.String]bencode.BenType{
//			*bencode.NewString("interval"): bencode.NewInteger(900),
//			*bencode.NewString("peers"):    peers,
//		},
//	)
//	buf := &bytes.Buffer{}
//	err := bencode.Encode(buf, []bencode.BenType{response})
//	require.NoError(t, err)
//
//	announcer := pkg.NewMockAnnouncer(t)
//	readCloser := &body{Buffer: buf}
//	announcer.EXPECT().Do(mock.Anything).Return(&http.Response{Body: readCloser}, nil)
//	torrentFile, err := torrent.Open("../testdata/cat.png.torrent", "../testdata/downloads/local")
//	require.NoError(t, err)
//	s := storage.NewStorage()
//	err = s.Set(torrentFile.InfoHash, torrentFile)
//	require.NoError(t, err)
//	return NewClient(peer.ID([]byte("-GO0001-local_peer00")), 6882, s, announcer)
//}
//
//func createRemoteClient(t *testing.T) *Client {
//	peers := bencode.NewString("")
//	response := bencode.NewDictionary(
//		map[bencode.String]bencode.BenType{
//			*bencode.NewString("interval"): bencode.NewInteger(900),
//			*bencode.NewString("peers"):    peers,
//		},
//	)
//	buf := &bytes.Buffer{}
//	err := bencode.Encode(buf, []bencode.BenType{response})
//	require.NoError(t, err)
//
//	announcer := pkg.NewMockAnnouncer(t)
//	readCloser := &body{Buffer: buf}
//	announcer.EXPECT().Do(mock.Anything).Return(&http.Response{Body: readCloser}, nil)
//	torrentFile, err := torrent.Open("../testdata/cat.png.torrent", "../testdata/downloads/remote")
//	require.NoError(t, err)
//	s := storage.NewStorage()
//	err = s.Set(torrentFile.InfoHash, torrentFile)
//	require.NoError(t, err)
//	return NewClient(peer.ID([]byte("-GO0001-remote_peer0")), 6881, s, announcer)
//}
//
//type body struct {
//	*bytes.Buffer
//}
//
//func (b *body) Close() error {
//	return nil
//}

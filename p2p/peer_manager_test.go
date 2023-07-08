package p2p

import (
	"testing"
)

func TestPeerManager_Run(t *testing.T) {
	// TODO:
	//ctx, cancel := context.WithCancel(context.Background())
	//defer cancel()
	//
	//file, err := os.Open("../testdata/debian-12.0.0-amd64-netinst.iso.torrent")
	//if err != nil {
	//	log.Fatal(err)
	//}
	//defer file.Close()
	//torrent, err := Open(file)
	//if err != nil {
	//	log.Fatal(err)
	//}
	//
	//peer := Peer{
	//	IP:       net.IP{127, 0, 0, 1},
	//	Port:     listenPort,
	//	InfoHash: &torrent.InfoHash,
	//}
	//// remote peer
	//storage := NewStorage()
	//storage.Set(torrent.InfoHash, torrent)
	//c := NewClient(storage)
	//var wg sync.WaitGroup
	//wg.Add(2)
	//go c.listen(ctx, &wg)
	//go c.managePeers(ctx, &wg)
	//
	//time.Sleep(500 * time.Millisecond) // wait till the listener is started
	//
	//pm := NewPeerManager(storage, peer, nil)
	//
	//// me
	//wg.Add(1)
	//go pm.Run(ctx, &wg)
	//cancel()
	//wg.Wait()
}

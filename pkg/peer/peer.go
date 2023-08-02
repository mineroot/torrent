package peer

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
)

const IdSize = 20

type ID [IdSize]byte

// IDFromString panics if str isn't IdSize bytes long
func IDFromString(str string) ID {
	return ID([]byte(str))
}

func (id ID) PeerId() [IdSize]byte {
	return id
}

type Peers []Peer

type Peer struct {
	InfoHash torrent.Hash
	IP       net.IP
	Port     uint16
	Conn     net.Conn
	//torrent *torrent.File // TODO maybe?
}

func (p *Peer) Address() string {
	return fmt.Sprintf("%s:%d", p.IP, p.Port)
}

func (p *Peer) sendHandshake(ctx context.Context, dialer ContextDialer, id ID) (err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		var err error
		if p.Conn, err = dialer.DialContext(ctx, "tcp", p.Address()); err != nil {
			errCh <- fmt.Errorf("unable to establish conn: %w", err)
			return
		}
		myHs := newHandshake(p.InfoHash, id)
		if _, err = p.Conn.Write(myHs.encode()); err != nil {
			errCh <- fmt.Errorf("unable to write handshake: %w", err)
			return
		}
		buf := make([]byte, handshakeLen)
		if _, err = io.ReadFull(p.Conn, buf); err != nil {
			errCh <- fmt.Errorf("unable to read handshake: %w", err)
			return
		}
		peerHs := newHandshake(p.InfoHash, ID{})
		if err = peerHs.decode(buf); err != nil {
			errCh <- fmt.Errorf("unable to decode handshake: %w", err)
			return
		}
	}()
	select {
	case <-ctx.Done():
		if p.Conn != nil {
			p.Conn.SetDeadline(time.Now())
		}
		err = ctx.Err()
	case err = <-errCh:
	}
	return
}

func (p *Peer) acceptHandshake(ctx context.Context, storage storage.Reader, id ID) (t *storage.TorrentData, err error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		defer close(errCh)
		buf := make([]byte, handshakeLen)
		if _, err := io.ReadFull(p.Conn, buf); err != nil {
			errCh <- fmt.Errorf("unable to read handshake: %w", err)
			return
		}
		peerHs := newHandshake(p.InfoHash, ID{})
		err := peerHs.decode(buf)
		if err != nil {
			errCh <- fmt.Errorf("unable to decode handshake: %w", err)
			return
		}
		p.InfoHash = peerHs.infoHash
		if t = storage.Get(p.InfoHash); t == nil {
			errCh <- fmt.Errorf("torrent with info hash %s not found", p.InfoHash)
			return
		}
		myHs := newHandshake(p.InfoHash, id)
		if _, err = p.Conn.Write(myHs.encode()); err != nil {
			errCh <- fmt.Errorf("unable to write handshake: %w", err)
		}
	}()
	select {
	case <-ctx.Done():
		p.Conn.SetDeadline(time.Now())
		err = ctx.Err()
	case err = <-errCh:
	}
	return
}

package peer

import (
	"github.com/stretchr/testify/assert"
	"testing"

	"github.com/mineroot/torrent/pkg/torrent"
)

func TestHandshake_encode_decode(t *testing.T) {
	hs := &handshake{
		infoHash: torrent.Hash([]byte("Hello, world! 01234 ")),
		peerID:   IDFromString("-GO0001-random_bytes"),
	}
	encoded := hs.encode()
	assert.Equal(t, "\x13BitTorrent protocol\x00\x00\x00\x00\x00\x00\x00\x00Hello, world! 01234 -GO0001-random_bytes", string(encoded))
	hsDecoded := &handshake{}
	err := hsDecoded.decode(encoded)
	assert.NoError(t, err)
	assert.Equal(t, hsDecoded, hs)
}

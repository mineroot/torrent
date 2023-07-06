package p2p

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestOpen(t *testing.T) {
	r, err := os.Open("../testdata/debian-12.0.0-amd64-netinst.iso.torrent")
	require.NoError(t, err)
	defer r.Close()
	torrent, err := Open(r)
	require.NoError(t, err)
	assert.Equal(t, "b851474b74f65cd19f981c723590e3e520242b97", torrent.InfoHash.String())
	assert.Equal(t, "http://bttracker.debian.org:6969/announce", torrent.Announce)
	assert.Equal(t, "debian-12.0.0-amd64-netinst.iso", torrent.Name)
	assert.Equal(t, 262144, torrent.PieceLength)
	assert.Equal(t, 773849088, torrent.Length)
	assert.Len(t, torrent.PieceHashes, 2952)
}

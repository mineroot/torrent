package torrent

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestOpenSingleFile(t *testing.T) {
	torrent, err := Open("../../testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
	require.NoError(t, err)
	assert.Equal(t, "b851474b74f65cd19f981c723590e3e520242b97", torrent.InfoHash.String())
	assert.Equal(t, "http://bttracker.debian.org:6969/announce", torrent.Announce)
	assert.Equal(t, 262144, torrent.PieceLength)
	assert.Equal(t, 2952, torrent.PiecesCount())
	require.Equal(t, 1, torrent.DownloadFilesCount())
	assert.Equal(t, "debian-12.0.0-amd64-netinst.iso", torrent.DownloadFiles[0].Name)
	assert.Equal(t, int64(773849088), torrent.DownloadFiles[0].Length)
	assert.Equal(t, int64(773849088), torrent.TotalLength())
}

func TestOpenMultipleFiles(t *testing.T) {
	torrent, err := Open("../../testdata/cats.torrent", "")
	require.NoError(t, err)
	assert.Equal(t, "a95c75a6224ec1432ec9923f22759d1e794565b2", torrent.InfoHash.String())
	assert.Equal(t, "http://127.0.0.1:8080/announce", torrent.Announce)
	assert.Equal(t, 131072, torrent.PieceLength)
	assert.Equal(t, 346, torrent.PiecesCount())
	require.Equal(t, 4, torrent.DownloadFilesCount())
	assert.Equal(t, "cat1.png", torrent.DownloadFiles[0].Name)
	assert.Equal(t, int64(11173362), torrent.DownloadFiles[0].Length)
	assert.Equal(t, "sub_dir/cat2.png", torrent.DownloadFiles[1].Name)
	assert.Equal(t, int64(12052984), torrent.DownloadFiles[1].Length)
	assert.Equal(t, "sub_dir/sub_sub_dir/cat3.png", torrent.DownloadFiles[2].Name)
	assert.Equal(t, int64(10914579), torrent.DownloadFiles[2].Length)
	assert.Equal(t, "sub_dir2/cat1_copy.png", torrent.DownloadFiles[3].Name)
	assert.Equal(t, int64(11173362), torrent.DownloadFiles[3].Length)
	assert.Equal(t, int64(45314287), torrent.TotalLength())
}

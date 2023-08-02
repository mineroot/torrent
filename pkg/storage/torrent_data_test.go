package storage

import (
	"bytes"
	"fmt"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"testing"

	"github.com/mineroot/torrent/pkg/torrent"
)

func TestNewTorrentData(t *testing.T) {
	torr, err := torrent.Open("../../testdata/cats.torrent", "download_dir")
	require.NoError(t, err)
	require.Equal(t, 4, torr.DownloadFilesCount())
	memFs := afero.NewMemMapFs()
	td, err := NewTorrentData(memFs, torr)
	require.NoError(t, err)
	require.NotNil(t, td)
	assert.False(t, td.bitfield.IsCompleted())
	assert.Equal(t, 0, td.bitfield.DownloadedPiecesCount())
	stat, err := memFs.Stat("download_dir/cat1.png")
	require.NoError(t, err)
	assert.Equal(t, int64(11173362), stat.Size())
	stat, err = memFs.Stat("download_dir/sub_dir/cat2.png")
	require.NoError(t, err)
	assert.Equal(t, int64(12052984), stat.Size())
	stat, err = memFs.Stat("download_dir/sub_dir/sub_sub_dir/cat3.png")
	require.NoError(t, err)
	assert.Equal(t, int64(10914579), stat.Size())
	stat, err = memFs.Stat("download_dir/sub_dir2/cat1_copy.png")
	require.NoError(t, err)
	assert.Equal(t, int64(11173362), stat.Size())
}

func TestTorrentData_WriteAt_ReadAt(t *testing.T) {
	type opExpect struct {
		p   []byte
		off int64
		n   int
		err error
	}
	type readWriteExpects struct {
		write *opExpect
		read  *opExpect
	}

	torr, err := torrent.Open("../../testdata/cats.torrent", "download_dir")
	require.NoError(t, err)
	memFs := afero.NewMemMapFs()
	td, err := NewTorrentData(memFs, torr)
	require.NoError(t, err)
	require.NotNil(t, td)

	formatStep := func(step int) string {
		return fmt.Sprintf("Error at step #%d", step)
	}

	// spans multiple files
	hugePayload := bytes.Repeat([]byte{0xAA}, int(td.sizes[0]+td.sizes[1]))

	wholeStreamPayload := bytes.Repeat([]byte{0xBB}, int(td.Torrent().TotalLength()))

	wholeStreamPayloadShifted := make([]byte, len(wholeStreamPayload))
	copy(wholeStreamPayloadShifted[100:], wholeStreamPayload[:len(wholeStreamPayload)-100])

	eraseAll := make([]byte, len(wholeStreamPayload))

	tests := []readWriteExpects{
		// write 3 times at the beginning shifting 1 byte right
		// 0x01, 0x02, 0x03, ----, ----
		// ----, 0x01, 0x02, 0x03, ----
		// ----, ----, 0x01, 0x02, 0x03
		// result should be:
		// 0x01, 0x01, 0x01, 0x02, 0x03
		0: {
			write: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 0,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 0,
				n:   3,
			},
		},
		1: {
			write: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 1,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 1,
				n:   3,
			},
		},
		2: {
			write: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 2,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0x01, 0x02, 0x03},
				off: 2,
				n:   3,
			},
		},
		3: {
			read: &opExpect{
				p:   []byte{0x1, 0x1, 0x1, 0x2, 0x3},
				off: 0,
				n:   5,
			},
		},
		// erase prev writes
		4: {
			write: &opExpect{
				p:   []byte{0x0, 0x0, 0x0, 0x0, 0x0},
				off: 0,
				n:   5,
			},
			read: &opExpect{
				p:   []byte{0x0, 0x0, 0x0, 0x0, 0x0},
				off: 0,
				n:   5,
			},
		},
		// write 3 times at the end of the first file shifting 1 byte right
		// 0xFD, 0xFE, 0xFF, | ----, ----
		// ----, 0xFD, 0xFE, | 0xFF, ----
		// ----, ----, 0xFD, | 0xFE, 0xFF
		// result should be:
		// 0xFD, 0xFD, 0xFD, | 0xFE, 0xFF
		5: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 3,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 3,
				n:   3,
			},
		},
		6: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 2,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 2,
				n:   3,
			},
		},
		7: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 1,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 1,
				n:   3,
			},
		},
		8: {
			read: &opExpect{
				p:   []byte{0xFD, 0xFD, 0xFD, 0xFE, 0xFF},
				off: torr.DownloadFiles[0].Length - 3,
				n:   5,
			},
		},
		// write 4 times at the end of the whole stream shifting 1 byte right
		// 0xFD, 0xFE, 0xFF | [EOF]
		// ----, 0xFD, 0xFE | [EOF]
		// ----, ----, 0xFD | [EOF]
		// ----, ----, ---- | [EOF]
		// result should be:
		// 0xFD, 0xFD, 0xFD | [EOF]
		9: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.TotalLength() - 3,
				n:   3,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.TotalLength() - 3,
				n:   3,
			},
		},
		10: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.TotalLength() - 2,
				n:   2,
				err: io.ErrShortWrite,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0xFE, 0x00},
				off: torr.TotalLength() - 2,
				n:   2,
				err: io.EOF,
			},
		},
		11: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.TotalLength() - 1,
				n:   1,
				err: io.ErrShortWrite,
			},
			read: &opExpect{
				p:   []byte{0xFD, 0x00, 0x00},
				off: torr.TotalLength() - 1,
				n:   1,
				err: io.EOF,
			},
		},
		12: {
			write: &opExpect{
				p:   []byte{0xFD, 0xFE, 0xFF},
				off: torr.TotalLength(),
				n:   0,
				err: io.ErrShortWrite,
			},
			read: &opExpect{
				p:   []byte{0x00, 0x00, 0x00},
				off: torr.TotalLength(),
				n:   0,
				err: io.EOF,
			},
		},
		13: {
			read: &opExpect{
				p:   []byte{0xFD, 0xFD, 0xFD, 0x00, 0x00},
				off: torr.TotalLength() - 3,
				n:   3,
				err: io.EOF,
			},
		},
		// negative offset
		14: {
			write: &opExpect{
				p:   []byte{0x00, 0x00, 0x00},
				off: -1,
				n:   0,
				err: io.ErrShortWrite,
			},
			read: &opExpect{
				p:   []byte{0x00, 0x00, 0x00},
				off: -1,
				n:   0,
				err: io.EOF,
			},
		},
		// payload spans multiple files (in case one piece covers multiple files)
		15: {
			write: &opExpect{
				p:   hugePayload,
				off: 100,
				n:   len(hugePayload),
			},
			read: &opExpect{
				p:   hugePayload,
				off: 100,
				n:   len(hugePayload),
			},
		},
		// whole stream
		16: {
			write: &opExpect{
				p:   wholeStreamPayload,
				off: 0,
				n:   len(wholeStreamPayload),
			},
			read: &opExpect{
				p:   wholeStreamPayload,
				off: 0,
				n:   len(wholeStreamPayload),
			},
		},
		// erase all
		17: {
			write: &opExpect{
				p:   eraseAll,
				off: 0,
				n:   len(eraseAll),
			},
			read: &opExpect{
				p:   eraseAll,
				off: 0,
				n:   len(eraseAll),
			},
		},
		18: {
			write: &opExpect{
				p:   wholeStreamPayload,
				off: 100,
				n:   len(wholeStreamPayload) - 100,
				err: io.ErrShortWrite,
			},
			read: &opExpect{
				p:   wholeStreamPayloadShifted,
				off: 0,
				n:   len(wholeStreamPayloadShifted),
			},
		},
	}

	for step, expects := range tests {
		if expects.write != nil {
			require.NotPanics(t, func() {
				n, err := td.WriteAt(expects.write.p, expects.write.off)
				assert.Equal(t, expects.write.n, n, formatStep(step))
				assert.ErrorIs(t, err, expects.write.err, formatStep(step))
			}, formatStep(step))
		}
		if expects.read != nil {
			require.NotPanics(t, func() {
				buf := make([]byte, len(expects.read.p))
				n, err := td.ReadAt(buf, expects.read.off)
				assert.Equal(t, expects.read.n, n, formatStep(step))
				assert.Equal(t, expects.read.p, buf, formatStep(step))
				assert.ErrorIs(t, err, expects.read.err, formatStep(step))
			}, formatStep(step))
		}

	}

}

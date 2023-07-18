package download

import (
	"bytes"
	"context"
	"crypto/sha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"io"
	"math/rand"
	"sync"
	"testing"
	"time"
	"torrent/p2p/bitfield"
	"torrent/p2p/divide"
	"torrent/p2p/torrent"
)

const (
	totalSize   = 773849088
	pieceSize   = 262144
	piecesCount = totalSize / pieceSize
)

func TestManager(t *testing.T) {
	reader, hashes := setup()

	items := divide.Divide(totalSize, []int{pieceSize, BlockSize})
	bf := bitfield.New(piecesCount)
	assert.False(t, bf.IsCompleted())
	m := newManager(items, bf)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blockHandlers := 100
	var wg sync.WaitGroup
	wg.Add(blockHandlers)

	for i := 0; i < blockHandlers; i++ {
		go func() {
			defer wg.Done()
			for {
				block, err := m.GenerateBlock(ctx)
				if err != nil {
					require.ErrorIs(t, err, ErrNoMoreBlocks)
					return
				}
				assert.NotZero(t, block)
				time.Sleep(2 * time.Millisecond)
			}
		}()
	}

	itemsFromRemotePeer := divide.Divide(totalSize, []int{pieceSize, BlockSize})
	wg.Add(blockHandlers)
	for i := 0; i < blockHandlers; i++ {
		go func() {
			defer wg.Done()
			for {
				item, ok := <-itemsFromRemotePeer
				if !ok {
					return
				}
				block := Block{
					PieceIndex: item.ParentIndex,
					Begin:      item.Begin,
					Len:        item.Len,
				}

				pieceVerified, err := m.MarkAsDownloaded(block, reader, hashes[block.PieceIndex], pieceSize)
				require.NoError(t, err)
				if pieceVerified {
					assert.True(t, bf.Has(block.PieceIndex))
				}
			}
		}()
	}

	wg.Wait()
	assert.True(t, bf.IsCompleted())
}

func setup() (io.ReaderAt, []torrent.Hash) {
	buf := make([]byte, 773849088)
	_, _ = rand.Read(buf)
	hashes := make([]torrent.Hash, 0, piecesCount)
	for i := 0; i < piecesCount; i++ {
		from := i * pieceSize
		to := from + pieceSize
		hashes = append(hashes, sha1.Sum(buf[from:to]))
	}
	return bytes.NewReader(buf), hashes
}

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

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/divide"
	"github.com/mineroot/torrent/pkg/torrent"
)

const (
	totalSize   = 773849088
	pieceSize   = 262144
	piecesCount = totalSize / pieceSize
)

func TestBlockGenerator(t *testing.T) {
	reader, hashes := setup()

	items := divide.Divide(totalSize, []int{pieceSize, BlockSize})
	bf := bitfield.New(piecesCount)
	assert.False(t, bf.IsCompleted())
	bg := newBlockGenerator(items, bf)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	blockHandlers := 100
	var wg sync.WaitGroup
	wg.Add(blockHandlers)

	for i := 0; i < blockHandlers; i++ {
		go func() {
			defer wg.Done()
			for {
				block, err := bg.Generate(ctx)
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
				block := NewBlock(item.ParentIndex, item.Begin, item.Len)

				pieceVerified, err := bg.MarkAsDownloaded(block, reader, hashes[block.PieceIndex], pieceSize)
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

func TestBlockGenerators_Load(t *testing.T) {
	items := divide.Divide(totalSize, []int{pieceSize, BlockSize})
	bf := bitfield.New(piecesCount)
	assert.False(t, bf.IsCompleted())
	bg1 := newBlockGenerator(items, bf)
	bg2 := newBlockGenerator(items, bf)

	hashable1 := hashable{hash: torrent.Hash{1}}
	hashable2 := hashable{hash: torrent.Hash{2}}
	hashable3 := hashable{hash: torrent.Hash{3}}

	bgs := &BlockGenerators{}
	bgs.syncMap.Store(hashable1.hash, bg1)
	bgs.syncMap.Store(hashable2.hash, bg2)

	assert.Equal(t, bg1, bgs.Load(hashable1))
	assert.Equal(t, bg2, bgs.Load(hashable2))
	assert.NotEqual(t, bg1, bgs.Load(hashable2))
	assert.Nil(t, bgs.Load(hashable3))
}

type hashable struct {
	hash torrent.Hash
}

func (h hashable) GetHash() torrent.Hash {
	return h.hash
}

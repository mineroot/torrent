package download

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/divide"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
)

const BlockSize = 1 << 14 // 16kB

var ErrNoMoreBlocks = fmt.Errorf("no more blocks")

type BlockGenerators struct {
	syncMap sync.Map
}

func (m *BlockGenerators) Load(hashable torrent.Hashable) *BlockGenerator {
	blockGenerator, ok := m.syncMap.Load(hashable.GetHash())
	if !ok {
		// TODO: maybe panic?
		return nil
	}
	return blockGenerator.(*BlockGenerator)
}

func CreateBlockGenerators(storage storage.Reader) *BlockGenerators {
	blockGenerators := &BlockGenerators{}
	for td := range storage.Iterator() {
		blocks := divide.Divide(int(td.Torrent().TotalLength()), []int{td.Torrent().PieceLength, BlockSize})
		blockGenerators.syncMap.Store(td.InfoHash(), newBlockGenerator(blocks, td.Bitfield()))
	}
	return blockGenerators
}

type BlockGenerator struct {
	bitfield      *bitfield.Bitfield
	blocksByPiece map[int]BlocksMap
	blocksQ       chan Block

	lock             sync.RWMutex
	downloadedBlocks BlocksMap
}

func newBlockGenerator(items <-chan divide.Item, bitfield *bitfield.Bitfield) *BlockGenerator {
	blocksByPiece := make(map[int]BlocksMap, bitfield.PiecesCount())
	blocksCount := 0
	for item := range items {
		if !bitfield.Has(item.ParentIndex) {
			blocks, ok := blocksByPiece[item.ParentIndex]
			if !ok {
				blocks = make(BlocksMap)
				blocksByPiece[item.ParentIndex] = blocks
			}
			blocks.Add(NewBlock(item.ParentIndex, item.Begin, item.Len))
			blocksCount++
		}
	}

	blocksQ := make(chan Block, blocksCount)
	for _, blocks := range blocksByPiece {
		for block := range blocks {
			blocksQ <- block
		}
	}
	if cap(blocksQ) == 0 {
		close(blocksQ)
	}
	return &BlockGenerator{
		bitfield:         bitfield,
		blocksByPiece:    blocksByPiece,
		blocksQ:          blocksQ,
		downloadedBlocks: make(BlocksMap),
	}
}

func (m *BlockGenerator) Generate(ctx context.Context) (Block, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	for {
		select {
		case <-ctx.Done():
			return Block{}, ctx.Err()
		case block, ok := <-m.blocksQ:
			if !ok {
				return Block{}, ErrNoMoreBlocks
			}
			if !m.downloadedBlocks.Has(block) { // block isn't downloaded yet
				// put it back to queue and return it
				m.blocksQ <- block
				return block, nil
			}
			// block is downloaded
			// read from chan again on next iteration until finding a not completed block
		}
	}
}

func (m *BlockGenerator) MarkAsDownloaded(
	block Block,
	r io.ReaderAt,
	hash torrent.Hash,
	pieceSize int,
) (pieceVerified bool, err error) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if !m.hasBlock(block) {
		return
	}
	if m.downloadedBlocks.Has(block) {
		return
	}
	// mark block as downloaded
	m.downloadedBlocks.Add(block)
	// check if a piece is downloaded
	pieceDownloaded := true
	for block = range m.blocksByPiece[block.PieceIndex] {
		if !m.downloadedBlocks.Has(block) {
			pieceDownloaded = false
			break
		}
	}
	if !pieceDownloaded {
		return
	}

	pieceVerified, err = torrent.VerifyPiece(r, hash, block.PieceIndex, pieceSize)
	if err != nil {
		return
	}
	if !pieceVerified {
		// if piece's hash doesn't match
		for block = range m.blocksByPiece[block.PieceIndex] {
			// delete all piece's blocks from a map and put them back to queue
			m.downloadedBlocks.Delete(block)
			m.blocksQ <- block
		}
		return
	}

	// if a piece has zero block(s), they may not have downloaded yet, but a piece has already been verified
	// in this case mark all remaining piece's blocks as downloaded
	for block = range m.blocksByPiece[block.PieceIndex] {
		m.downloadedBlocks.Add(block)
	}

	if err = m.bitfield.Set(block.PieceIndex); err != nil {
		panic(fmt.Errorf("download: %w", err))
	}

	if m.bitfield.IsCompleted() {
		close(m.blocksQ)
	}
	return
}

func (m *BlockGenerator) hasBlock(block Block) bool {
	blocks, ok := m.blocksByPiece[block.PieceIndex]
	if !ok {
		return false
	}
	return blocks.Has(block)
}

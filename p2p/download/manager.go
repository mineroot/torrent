package download

import (
	"context"
	"fmt"
	"io"
	"sync"
	"torrent/p2p/bitfield"
	"torrent/p2p/divide"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
)

const BlockSize = 1 << 14 // 16kB

var ErrNoMoreBlocks = fmt.Errorf("no more blocks")

type Managers struct {
	syncMap sync.Map
}

func (m *Managers) Load(hashable torrent.Hashable) *Manager {
	manager, _ := m.syncMap.Load(hashable.GetHash())
	return manager.(*Manager)
}

func CreateDownloadManagers(storage storage.Reader) *Managers {
	managers := &Managers{}
	for t := range storage.Iterator() {
		bf := storage.GetBitfield(t.InfoHash)
		blocks := divide.Divide(t.Length, []int{t.PieceLength, BlockSize})
		managers.syncMap.Store(t.InfoHash, newManager(blocks, bf))
	}
	return managers
}

type Block struct {
	PieceIndex int
	Begin      int
	Len        int
}

type Manager struct {
	bitfield      *bitfield.Bitfield
	blocksByPiece map[int]map[Block]struct{}
	blocksQ       chan Block

	lock             sync.RWMutex
	downloadedBlocks map[Block]struct{}
}

func newManager(items <-chan divide.Item, bitfield *bitfield.Bitfield) *Manager {
	blocksByPiece := make(map[int]map[Block]struct{}, bitfield.PiecesCount())

	blocksCount := 0
	for item := range items {
		if !bitfield.Has(item.ParentIndex) {
			blocks, ok := blocksByPiece[item.ParentIndex]
			if !ok {
				blocks = make(map[Block]struct{})
				blocksByPiece[item.ParentIndex] = blocks
			}
			blocks[Block{
				PieceIndex: item.ParentIndex,
				Begin:      item.Begin,
				Len:        item.Len,
			}] = struct{}{}
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
	return &Manager{
		bitfield:         bitfield,
		blocksByPiece:    blocksByPiece,
		blocksQ:          blocksQ,
		downloadedBlocks: make(map[Block]struct{}),
	}
}

func (m *Manager) GenerateBlock(ctx context.Context) (Block, error) {
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
			if _, ok := m.downloadedBlocks[block]; !ok { // block isn't downloaded yet
				// put it back to queue and return it
				m.blocksQ <- block
				return block, nil
			}
			// block is downloaded
			// read from chan again on next iteration until finding a not completed block
		}
	}
}

func (m *Manager) MarkAsDownloaded(
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
	if _, blockDownloaded := m.downloadedBlocks[block]; blockDownloaded {
		return
	}
	m.downloadedBlocks[block] = struct{}{}
	pieceDownloaded := true
	for block = range m.blocksByPiece[block.PieceIndex] {
		if _, blockDownloaded := m.downloadedBlocks[block]; !blockDownloaded {
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
			delete(m.downloadedBlocks, block)
			m.blocksQ <- block
		}
		return
	}

	// if a piece has zero block(s), they may not have downloaded yet, but a piece has already been verified
	// in this case mark all remaining piece's blocks as downloaded
	for block = range m.blocksByPiece[block.PieceIndex] {
		m.downloadedBlocks[block] = struct{}{}
	}

	m.bitfield.Set(block.PieceIndex)
	if m.bitfield.IsCompleted() {
		close(m.blocksQ)
	}
	return
}

func (m *Manager) hasBlock(block Block) bool {
	blocks, ok := m.blocksByPiece[block.PieceIndex]
	if !ok {
		return false
	}
	_, ok = blocks[block]
	return ok
}

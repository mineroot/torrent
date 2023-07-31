package download

import (
	"sync"
	"time"
)

type Block struct {
	PieceIndex int
	Begin      int
	Len        int
}

func NewBlock(pieceIndex int, begin int, len int) Block {
	return Block{PieceIndex: pieceIndex, Begin: begin, Len: len}
}

type BlocksMap map[Block]struct{}

func (m BlocksMap) Add(block Block) {
	m[block] = struct{}{}
}

func (m BlocksMap) Has(block Block) bool {
	_, ok := m[block]
	return ok
}

func (m BlocksMap) Delete(block Block) {
	delete(m, block)
}

type BlocksSyncMap struct {
	mu sync.RWMutex
	m  map[Block]time.Time
}

func NewBlocksSyncMap() *BlocksSyncMap {
	return &BlocksSyncMap{m: make(map[Block]time.Time)}
}

func (m *BlocksSyncMap) Add(block Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.m[block] = time.Now()
}

func (m *BlocksSyncMap) Delete(block Block) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.m, block)
}

func (m *BlocksSyncMap) Iterate() <-chan Block {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ch := make(chan Block, len(m.m))
	defer close(ch)
	for block := range m.m {
		ch <- block
	}
	return ch
}

func (m *BlocksSyncMap) LenNonExpired(expirationDur time.Duration) int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	now := time.Now()
	lenNonExpired := 0
	for _, t := range m.m {
		if t.Add(expirationDur).After(now) {
			lenNonExpired++
		}
	}
	return lenNonExpired
}

func (m *BlocksSyncMap) Has(block Block) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.m[block]
	return ok
}

func (m *BlocksSyncMap) Len() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.m)
}

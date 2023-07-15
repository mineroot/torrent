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

var ErrNoMoreTasks = fmt.Errorf("no more tasks")

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

type Task struct {
	PieceIndex int
	Begin      int
	Len        int
}

type Manager struct {
	tasksQ   chan Task
	bitfield *bitfield.Bitfield
	tasksNum int
	allTasks map[Task]struct{}

	lock           sync.RWMutex
	completedTasks map[Task]struct{}
	verifiedPieces map[int]struct{}
}

func newManager(items <-chan divide.Item, bitfield *bitfield.Bitfield) *Manager {
	approxBlocksCap := bitfield.PiecesCount() * 16
	allTasks := make(map[Task]struct{}, approxBlocksCap)

	for item := range items {
		if !bitfield.Has(item.ParentIndex) {
			allTasks[Task{
				PieceIndex: item.ParentIndex,
				Begin:      item.Begin,
				Len:        item.Len,
			}] = struct{}{}
		}
	}

	tasksQ := make(chan Task, len(allTasks))
	for task := range allTasks {
		tasksQ <- task
	}
	if cap(tasksQ) == 0 {
		close(tasksQ)
	}
	return &Manager{
		tasksQ:         tasksQ,
		bitfield:       bitfield,
		tasksNum:       len(allTasks),
		allTasks:       allTasks,
		completedTasks: make(map[Task]struct{}),
		verifiedPieces: make(map[int]struct{}),
	}
}

func (m *Manager) GenerateTask(ctx context.Context) (Task, error) {
	for {
		select {
		case <-ctx.Done():
			return Task{}, ctx.Err()
		case task, ok := <-m.tasksQ:
			if !ok {
				return Task{}, ErrNoMoreTasks
			}

			m.lock.RLock()
			_, taskOk := m.completedTasks[task]
			_, pieceOk := m.verifiedPieces[task.PieceIndex]
			if !taskOk && !pieceOk {
				// the task isn't completed, and it's piece isn't verified,
				// put it back to queue and return it
				m.tasksQ <- task
				m.lock.RUnlock()
				return task, nil
			}
			m.lock.RUnlock()
			// the task is completed
			// read from chan again on next iteration until finding a not completed task
		}
	}
}

func (m *Manager) CompleteTask(
	task Task,
	file io.ReaderAt,
	hash torrent.Hash,
	pieceLength int,
) (pieceVerified bool, err error) {
	if _, ok := m.allTasks[task]; !ok {
		return
	}
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.completedTasks[task]; !ok {
		m.completedTasks[task] = struct{}{}
	}
	ok, err := torrent.VerifyPiece(file, hash, task.PieceIndex, pieceLength)
	if err != nil {
		return
	}
	if ok && m.bitfield.Set(task.PieceIndex) {
		pieceVerified = true
		m.verifiedPieces[task.PieceIndex] = struct{}{}
		if m.bitfield.IsCompleted() {
			close(m.tasksQ)
		}
	}
	return
}

package download

import (
	"context"
	"fmt"
	"sync"
	"torrent/p2p/bitfield"
	"torrent/p2p/divide"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
)

const BlockLen = 1 << 14 // 16kB

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
		blocks := divide.Divide(t.Length, []int{t.PieceLength, BlockLen})
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
	downloadTasks  chan Task
	bitfield       *bitfield.Bitfield
	tasksNum       int
	lock           sync.RWMutex
	completedTasks map[Task]struct{}
}

func newManager(items <-chan divide.Item, bitfield *bitfield.Bitfield) *Manager {
	approxBlocksCount := bitfield.PiecesCount() * 16
	tasksTmp := make([]Task, 0, approxBlocksCount)

	for item := range items {
		if !bitfield.Has(item.ParentIndex) {
			tasksTmp = append(tasksTmp, Task{
				PieceIndex: item.ParentIndex,
				Begin:      item.Begin,
				Len:        item.Len,
			})
		}
	}

	downloadTasks := make(chan Task, len(tasksTmp))
	for _, task := range tasksTmp {
		downloadTasks <- task
	}
	return &Manager{
		downloadTasks:  downloadTasks,
		bitfield:       bitfield,
		tasksNum:       len(tasksTmp),
		completedTasks: make(map[Task]struct{}),
	}
}

func (m *Manager) GenerateTask(ctx context.Context) (Task, error) {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if len(m.completedTasks) == m.tasksNum {
		return Task{}, ErrNoMoreTasks
	}
	for {
		select {
		case task := <-m.downloadTasks:
			// task not completed - put it back to queue and return it
			if _, ok := m.completedTasks[task]; !ok {
				m.downloadTasks <- task
				return task, nil
			}
		case <-ctx.Done():
			return Task{}, ctx.Err()
		}
	}
}

func (m *Manager) CompleteTask(t Task) {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.completedTasks[t]; !ok {
		m.completedTasks[t] = struct{}{}
	}
}

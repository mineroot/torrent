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
	downloadTasks  chan Task
	bitfield       *bitfield.Bitfield // TODO remove?
	tasksNum       int
	lock           sync.RWMutex
	completedTasks map[Task]struct{}
}

func newManager(items <-chan divide.Item, bitfield *bitfield.Bitfield) *Manager {
	approxBlocksCap := bitfield.PiecesCount() * 16
	tasksTmp := make([]Task, 0, approxBlocksCap)

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
	if cap(downloadTasks) == 0 {
		close(downloadTasks)
	}
	return &Manager{
		downloadTasks:  downloadTasks,
		bitfield:       bitfield,
		tasksNum:       len(tasksTmp),
		completedTasks: make(map[Task]struct{}),
	}
}

func (m *Manager) GenerateTask(ctx context.Context) (Task, error) {
	for {
		select {
		case <-ctx.Done():
			return Task{}, ctx.Err()
		case task, ok := <-m.downloadTasks:
			if !ok {
				return Task{}, ErrNoMoreTasks
			}

			m.lock.RLock()
			_, ok = m.completedTasks[task]
			if !ok {
				// the task isn't completed
				// put it back to queue and return it
				m.downloadTasks <- task
				m.lock.RUnlock()
				return task, nil
			}
			m.lock.RUnlock()
			// the task is completed
			// read from chan again on next iteration until finding a not completed task
		}
	}
}

func (m *Manager) CompleteTask(t Task) int {
	m.lock.Lock()
	defer m.lock.Unlock()
	if _, ok := m.completedTasks[t]; !ok {
		m.completedTasks[t] = struct{}{}
	}
	completedTasksNum := len(m.completedTasks)
	if completedTasksNum == m.tasksNum {
		close(m.downloadTasks) // TODO what if already closed in newManager
	}
	return completedTasksNum
}

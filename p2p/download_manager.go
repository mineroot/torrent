package p2p

import (
	"fmt"
	"sync"
	"torrent/bitfield"
	"torrent/p2p/divide"
)

type downloadTask struct {
	pieceIndex int
	begin      int
	len        int
}

type downloadManager struct {
	downloadTasks  chan downloadTask
	bitfield       *bitfield.Bitfield
	lock           sync.RWMutex
	completedTasks map[downloadTask]struct{}
}

func newDownloadManager(items <-chan divide.Item, bitfield *bitfield.Bitfield) *downloadManager {
	dm := &downloadManager{
		//downloadTasks:  make(chan downloadTask, 2240), // todo 2240
		downloadTasks:  make(chan downloadTask, 47232),
		bitfield:       bitfield,
		completedTasks: make(map[downloadTask]struct{}),
	}
	go func() {
		for item := range items {
			if !bitfield.Has(item.ParentIndex) {
				dm.downloadTasks <- downloadTask{
					pieceIndex: item.ParentIndex,
					begin:      item.Begin,
					len:        item.Len,
				}
			}
		}
		//close(dm.downloadTasks)
	}()
	return dm
}

func (dm *downloadManager) generateTask() (downloadTask, error) {
	dm.lock.RLock()
	defer dm.lock.RUnlock()
	for {
		// get task from queue
		task, ok := <-dm.downloadTasks
		if !ok {
			return downloadTask{}, fmt.Errorf("no more download tasks")
		}
		// task not completed - put it back to queue and return it
		if _, ok = dm.completedTasks[task]; !ok {
			dm.downloadTasks <- task
			return task, nil
		}
		// if task id completed get next task in next iteration
	}
}

func (dm *downloadManager) completeTask(dt downloadTask) {
	dm.lock.Lock()
	defer dm.lock.Unlock()
	dm.completedTasks[dt] = struct{}{}
}

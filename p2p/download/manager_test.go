package download

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"sync"
	"testing"
	"torrent/p2p/bitfield"
	"torrent/p2p/divide"
)

func TestNewManager(t *testing.T) {
	totalSize, pieceSize, blockSize := 773849088, 262144, BlockSize
	piecesCount := totalSize / pieceSize
	if totalSize%pieceSize != 0 {
		piecesCount++
	}

	items := divide.Divide(totalSize, []int{pieceSize, blockSize})
	bf := bitfield.New(piecesCount)
	m := newManager(items, bf)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	taskHandlers := 10
	var wg sync.WaitGroup
	wg.Add(taskHandlers)
	for i := 0; i < taskHandlers; i++ {
		go func() {
			defer wg.Done()
			defer fmt.Println("DONE")
			for {
				task, err := m.GenerateTask(ctx)
				if err != nil {
					assert.ErrorIs(t, err, ErrNoMoreTasks)
					return
				}
				assert.NotZero(t, task)
			}
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		for item := range divide.Divide(totalSize, []int{pieceSize, blockSize}) {
			task := Task{
				PieceIndex: item.ParentIndex,
				Begin:      item.Begin,
				Len:        item.Len,
			}
			m.CompleteTask(task)
		}
	}()
	wg.Wait()
}

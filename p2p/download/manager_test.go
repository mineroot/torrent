package download

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	//totalSize, pieceSize, blockSize := 100, 24, 5
	//piecesCount := totalSize / pieceSize
	//if totalSize%pieceSize != 0 {
	//	piecesCount++
	//}
	//
	//items := divide.Divide(totalSize, []int{pieceSize, blockSize})
	//bf := bitfield.New(piecesCount)
	//m := newManager(items, bf)
	//
	//ctx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	//defer cancel()
	//task, err := m.GenerateTask(ctx)
	//assert.NoError(t, err)
	//
	//m.CompleteTask(task)
	//_, _ = m.GenerateTask(ctx)

}

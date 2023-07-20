package divide

type Item struct {
	ParentIndex int
	Index       int
	Begin       int
	Len         int
}

// Divide is a generic function to recursively divide totalSize into smaller items of given sizes,
// in our case, we'll use is to divide torrent files' total size
// first into pieces than pieces into blocks.
// Returns unbuffered chan of items, so reader should read all items to prevent goroutine leak
func Divide(totalSize int, sizes []int) <-chan Item {
	items := make(chan Item)
	go func() {
		divide(items, 0, totalSize, sizes)
		close(items)
	}()
	return items
}

func divide(items chan<- Item, parentIndex int, totalSize int, sizes []int) {
	if len(sizes) == 0 {
		panic("empty sizes")
	}
	size := sizes[0]
	sizes = sizes[1:]
	itemsNum := totalSize / size
	if totalSize%size != 0 {
		itemsNum++
	}
	for i := 0; i < itemsNum; i++ {
		newSize := size
		isLastItem := i == itemsNum-1
		if isLastItem {
			newSize = totalSize - i*newSize
		}
		if len(sizes) != 0 {
			divide(items, i, newSize, sizes)
		} else {
			items <- Item{
				ParentIndex: parentIndex,
				Index:       i,
				Begin:       i * size,
				Len:         newSize,
			}
		}
	}
}

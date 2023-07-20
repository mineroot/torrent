package divide

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestDivide(t *testing.T) {
	items := Divide(100, []int{24, 5})
	itemsSlice := make([]Item, 0)
	for item := range items {
		itemsSlice = append(itemsSlice, item)
	}
	expected := []Item{
		{ParentIndex: 0, Index: 0, Begin: 0, Len: 5},
		{ParentIndex: 0, Index: 1, Begin: 5, Len: 5},
		{ParentIndex: 0, Index: 2, Begin: 10, Len: 5},
		{ParentIndex: 0, Index: 3, Begin: 15, Len: 5},
		{ParentIndex: 0, Index: 4, Begin: 20, Len: 4},
		{ParentIndex: 1, Index: 0, Begin: 0, Len: 5},
		{ParentIndex: 1, Index: 1, Begin: 5, Len: 5},
		{ParentIndex: 1, Index: 2, Begin: 10, Len: 5},
		{ParentIndex: 1, Index: 3, Begin: 15, Len: 5},
		{ParentIndex: 1, Index: 4, Begin: 20, Len: 4},
		{ParentIndex: 2, Index: 0, Begin: 0, Len: 5},
		{ParentIndex: 2, Index: 1, Begin: 5, Len: 5},
		{ParentIndex: 2, Index: 2, Begin: 10, Len: 5},
		{ParentIndex: 2, Index: 3, Begin: 15, Len: 5},
		{ParentIndex: 2, Index: 4, Begin: 20, Len: 4},
		{ParentIndex: 3, Index: 0, Begin: 0, Len: 5},
		{ParentIndex: 3, Index: 1, Begin: 5, Len: 5},
		{ParentIndex: 3, Index: 2, Begin: 10, Len: 5},
		{ParentIndex: 3, Index: 3, Begin: 15, Len: 5},
		{ParentIndex: 3, Index: 4, Begin: 20, Len: 4},
		{ParentIndex: 4, Index: 0, Begin: 0, Len: 4},
	}
	assert.Equal(t, expected, itemsSlice)
}

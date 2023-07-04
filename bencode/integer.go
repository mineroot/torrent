package bencode

import (
	"fmt"
	"io"
	"strconv"
)

type Integer struct {
	val int64
}

func NewInteger(val int64) *Integer {
	return &Integer{val: val}
}

func (i *Integer) Encode(w io.Writer) error {
	encoded := fmt.Sprintf("i%de", i.val)
	_, err := w.Write([]byte(encoded))
	return err
}

func (i *Integer) String() string {
	return strconv.FormatInt(i.val, 10)
}

package bencode

import (
	"fmt"
	"io"
)

type String struct {
	val string
}

func NewString(val string) *String {
	return &String{val: val}
}

func (s *String) Encode(w io.Writer) error {
	encoded := fmt.Sprintf("%d:%s", len(s.val), s.val)
	_, err := w.Write([]byte(encoded))
	return err
}

func (s *String) String() string {
	return s.val
}

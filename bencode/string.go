package bencode

import (
	"fmt"
	"io"
	"unicode"
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
	for i := 0; i < len(s.val); i++ {
		if s.val[i] > unicode.MaxASCII {
			// print as bytes slice if not ascii
			return fmt.Sprintf("%v", []byte(s.val))
		}
	}
	return s.val
}

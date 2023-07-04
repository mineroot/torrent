package bencode

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
)

type Integer struct {
	val int64
}

func (i *Integer) Encode(w io.Writer) error {
	encoded := fmt.Sprintf("i%de", i.val)
	_, err := w.Write([]byte(encoded))
	return err
}

func (i *Integer) Decode(r io.Reader) error {
	reader := bufio.NewReader(r)
	char, _, err := reader.ReadRune()
	if err != nil {
		return err
	}
	if char != 'i' {
		return fmt.Errorf("'i' expected, got %c", char)
	}
	valString, err := reader.ReadString(byte('e'))
	if err != nil {
		return err
	}
	val, err := strconv.Atoi(valString[:len(valString)-1])
	if err != nil {
		return fmt.Errorf("integer expected, got %s: %w", valString, err)
	}
	i.val = int64(val)
	return nil
}

func NewInteger(val int64) *Integer {
	return &Integer{val: val}
}

package bencode

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
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

func (s *String) Decode(r io.Reader) error {
	reader := bufio.NewReader(r)
	lenString, err := reader.ReadString(byte(':'))
	if err != nil {
		return err
	}
	length, err := strconv.Atoi(lenString[:len(lenString)-1])
	if err != nil {
		return fmt.Errorf("string length expected, got %s: %w", lenString, err)
	}
	str := make([]byte, length)
	_, err = io.ReadFull(reader, str)
	if err != nil {
		return err
	}
	s.val = string(str)
	return nil
}

package bencode

import (
	"bufio"
	"fmt"
	"io"
)

type List struct {
	val []BenType
}

func NewList(val []BenType) *List {
	return &List{val: val}
}

func (l *List) Encode(w io.Writer) error {
	_, err := w.Write([]byte("l"))
	if err != nil {
		return err
	}
	for _, benType := range l.val {
		if err = benType.Encode(w); err != nil {
			return err
		}
	}
	_, err = w.Write([]byte("e"))
	return err
}

func (l *List) Decode(r io.Reader) error {
	l.val = make([]BenType, 0, 8)
	reader := bufio.NewReader(r)
	level := 0
	expectList := true
	for {
		b, _, err := reader.ReadRune()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		switch b {
		case 'i':
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			integer := NewInteger(0)
			if err = integer.Decode(reader); err != nil {
				return err
			}
			l.val = append(l.val, integer)
		case 'l':
			if expectList {
				expectList = false
				level++
				break
			}
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			list := NewList(nil)
			if err = list.Decode(reader); err != nil {
				return err
			}
			l.val = append(l.val, list)
		case 'd':
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			dict := NewDictionary(nil)
			if err = dict.Decode(reader); err != nil {
				return err
			}
			l.val = append(l.val, dict)
		case 'e':
			level--
			if level == 0 {
				return nil
			}
			if level < 0 {
				return fmt.Errorf("unexpected end")
			}
		default:
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			str := NewString("")
			if err = str.Decode(reader); err != nil {
				return err
			}
			l.val = append(l.val, str)
		}
	}
}

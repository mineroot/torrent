package bencode

import (
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

func (l *List) Add(item BenType) {
	l.val = append(l.val, item)
}

func (l *List) String() (s string) {
	for _, benType := range l.val {
		s += fmt.Sprintf("\t%s\n", benType)
	}
	return
}

func (l *List) printTree(indent, keyPrefix string) string {
	s := ""
	for i, benType := range l.val {
		switch t := benType.(type) {
		case *Dictionary:
			s += t.printTree(indent+"\t", "", t.val)
		case *Integer:
			s += fmt.Sprintf("%s%d\n", indent, t.val)
		case *String:
			s += fmt.Sprintf("%s%s\n", indent, t.String())
		case *List:
			s += t.printTree(indent+"\t", keyPrefix+fmt.Sprintf("[%d].", i))
		}
	}
	return s
}

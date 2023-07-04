package bencode

import (
	"fmt"
	"io"
	"sort"
)

type Dictionary struct {
	val map[String]BenType
}

func NewDictionary(val map[String]BenType) *Dictionary {
	return &Dictionary{val: val}
}

func (d *Dictionary) Encode(w io.Writer) error {
	_, err := w.Write([]byte("d"))
	if err != nil {
		return err
	}
	keys := make([]String, 0, len(d.val))
	for key := range d.val {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].val < keys[j].val
	})
	for _, key := range keys {
		err = key.Encode(w)
		if err != nil {
			return err
		}
		err = d.val[key].Encode(w)
		if err != nil {
			return err
		}
	}
	_, err = w.Write([]byte("e"))
	return err
}

func (d *Dictionary) Add(key String, value BenType) {
	d.val[key] = value
}

func (d *Dictionary) String() (s string) {
	return d.printTree("", "", d.val)
}

func (d *Dictionary) printTree(indent, keyPrefix string, val map[String]BenType) string {
	s := ""
	for key, benType := range val {
		s += fmt.Sprintf("%s%s\n", indent, keyPrefix+key.String())
		switch t := benType.(type) {
		case *Dictionary:
			s += t.printTree(indent+"\t", "", t.val)
		case *Integer:
			s += fmt.Sprintf("%s  %d\n", indent+"\t", t.val)
		case *String:
			s += fmt.Sprintf("%s  %s\n", indent+"\t", t.String())
		case *List:
			s += t.printTree(indent+"\t", keyPrefix+key.String()+".")
		}
	}
	return s
}

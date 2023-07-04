package bencode

import (
	"bufio"
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

func (d *Dictionary) Decode(r io.Reader) error {
	d.val = make(map[String]BenType)
	reader := bufio.NewReader(r)
	level := 0
	expectKey := true
	expectDict := true
	var currentKey String
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
			if expectKey {
				return fmt.Errorf("dict key expected, got %c", b)
			}
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			integer := NewInteger(0)
			if err = integer.Decode(reader); err != nil {
				return err
			}
			d.val[currentKey] = integer
			expectKey = !expectKey
		case 'l':
			if expectKey {
				return fmt.Errorf("dict key expected, got %c", b)
			}
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			list := NewList(nil)
			if err = list.Decode(reader); err != nil {
				return err
			}
			d.val[currentKey] = list
			expectKey = !expectKey
		case 'd':
			if expectDict { // detect first encounter,
				expectDict = false
				level++
				break
			}
			if expectKey {
				return fmt.Errorf("dict key expected, got %c", b)
			}
			if err = reader.UnreadByte(); err != nil {
				return err
			}
			dict := NewDictionary(nil)
			if err = dict.Decode(reader); err != nil {
				return err
			}
			d.val[currentKey] = dict
			expectKey = !expectKey
		case 'e':
			if expectKey && expectDict {
				return fmt.Errorf("dict key expected, got %c", b)
			}
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
			if expectKey {
				currentKey = *str
			} else {
				d.val[currentKey] = str
			}
			expectKey = !expectKey
		}
	}
}

package bencode

import (
	"bufio"
	"io"
)

type BenType interface {
	Encode(w io.Writer) error
	Decode(r io.Reader) error
}

func Encode(w io.Writer, data []BenType) error {
	for _, item := range data {
		err := item.Encode(w)
		if err != nil {
			return err
		}
	}
	return nil
}

func Decode(r io.Reader) ([]BenType, error) {
	reader := bufio.NewReader(r)
	decoded := make([]BenType, 0, 512)
	for {
		b, _, err := reader.ReadRune()
		if err == io.EOF {
			return decoded, nil
		}
		if err != nil {
			return nil, err
		}
		switch b {
		case 'i':
			if err = reader.UnreadByte(); err != nil {
				return nil, err
			}
			integer := NewInteger(0)
			if err = integer.Decode(reader); err != nil {
				return nil, err
			}
			decoded = append(decoded, integer)
		case 'l':
			if err = reader.UnreadByte(); err != nil {
				return nil, err
			}
			list := NewList(nil)
			if err = list.Decode(reader); err != nil {
				return nil, err
			}
			decoded = append(decoded, list)
		case 'd':
			if err = reader.UnreadByte(); err != nil {
				return nil, err
			}
			dict := NewDictionary(nil)
			if err = dict.Decode(reader); err != nil {
				return nil, err
			}
			decoded = append(decoded, dict)
		default:
			if err = reader.UnreadByte(); err != nil {
				return nil, err
			}
			str := NewString("")
			if err = str.Decode(reader); err != nil {
				return nil, err
			}
			decoded = append(decoded, str)
		}
	}
}

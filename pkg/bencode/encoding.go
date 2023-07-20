package bencode

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
)

type BenType interface {
	fmt.Stringer
	Encode(w io.Writer) error
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

func Decode(r io.Reader) (BenType, error) {
	reader := bufio.NewReader(r)
	b, err := reader.ReadByte()
	if err != nil {
		return nil, err
	}
	switch b {
	case 'i':
		intBuf, err := reader.ReadBytes('e')
		if err != nil {
			return nil, err
		}
		intBuf = intBuf[:len(intBuf)-1]
		integerVal, err := strconv.ParseInt(string(intBuf), 10, 64)
		if err != nil {
			return nil, err
		}
		return NewInteger(integerVal), nil
	case 'l':
		list := NewList([]BenType{})
		for {
			c, err := reader.ReadByte()
			if err == nil {
				if c == 'e' {
					return list, nil
				}
				_ = reader.UnreadByte()
			}
			value, err := Decode(reader)
			if err != nil {
				return nil, err
			}
			list.Add(value)
		}
	case 'd':
		dict := NewDictionary(map[String]BenType{})
		for {
			c, err := reader.ReadByte()
			if err == nil {
				if c == 'e' {
					return dict, nil
				}
				_ = reader.UnreadByte()
			}
			value, err := Decode(reader)
			if err != nil {
				return nil, err
			}
			key, ok := value.(*String)
			if !ok {
				return nil, errors.New("non-string dictionary key")
			}
			value, err = Decode(reader)
			if err != nil {
				return nil, err
			}
			dict.Add(*key, value)
		}
	default:
		_ = reader.UnreadByte()
		stringLengthBuffer, err := reader.ReadBytes(':')
		if err != nil {
			return nil, err
		}
		stringLengthBuffer = stringLengthBuffer[:len(stringLengthBuffer)-1]
		stringLength, err := strconv.ParseInt(string(stringLengthBuffer), 10, 64)
		if err != nil {
			return nil, err
		}
		buf := make([]byte, stringLength)
		_, err = io.ReadFull(reader, buf)
		return NewString(string(buf)), err
	}
}

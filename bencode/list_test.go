package bencode

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestList_Encode(t *testing.T) {
	tests := []struct {
		name    string
		decoded *List
		bencode string
	}{
		{
			name:    "empty list",
			decoded: NewList([]BenType{}),
			bencode: "le",
		},
		{
			name:    "list of string and int",
			decoded: NewList([]BenType{NewString("hello, world"), NewInteger(123)}),
			bencode: "l12:hello, worldi123ee",
		},
		{
			name:    "list of int and list",
			decoded: NewList([]BenType{NewInteger(123), NewList([]BenType{NewString("string in list"), NewInteger(-1)})}),
			bencode: "li123el14:string in listi-1eee",
		},
		{
			name: "list of string and dict",
			decoded: NewList([]BenType{NewString("test"), NewDictionary(map[String]BenType{
				*NewString("second"): NewString("str"),
				*NewString("first"):  NewInteger(123),
			})}),
			bencode: "l4:testd5:firsti123e6:second3:stree",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := tt.decoded.Encode(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.bencode, buf.String())

			list := NewList(nil)
			err = list.Decode(strings.NewReader(tt.bencode))
			require.NoError(t, err)
			assert.Equal(t, tt.decoded, list)
		})
	}
}

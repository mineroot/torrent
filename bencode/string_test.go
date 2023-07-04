package bencode

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestString_Encode_Decode(t *testing.T) {
	tests := []struct {
		name    string
		decoded *String
		bencode string
	}{
		{
			name:    "first",
			decoded: NewString("hello, world"),
			bencode: "12:hello, world",
		},
		{
			name:    "second",
			decoded: NewString("another string"),
			bencode: "14:another string",
		},
		{
			name:    "empty",
			decoded: NewString(""),
			bencode: "0:",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := tt.decoded.Encode(buf)
			assert.NoError(t, err)
			assert.Equal(t, tt.bencode, buf.String())
		})
	}
}

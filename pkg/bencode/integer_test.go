package bencode

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestInteger_Encode_Decode(t *testing.T) {
	tests := []struct {
		name    string
		decoded *Integer
		bencode string
	}{
		{
			name:    "positive",
			decoded: NewInteger(123),
			bencode: "i123e",
		},
		{
			name:    "negative",
			decoded: NewInteger(-123),
			bencode: "i-123e",
		},
		{
			name:    "zero",
			decoded: NewInteger(0),
			bencode: "i0e",
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

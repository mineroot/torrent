package bencode

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func TestEncode(t *testing.T) {
	tests := []struct {
		name     string
		data     []BenType
		expected string
	}{
		{
			name: "default",
			data: []BenType{
				NewString("hello"),
				NewInteger(123),
				NewDictionary(map[String]BenType{
					*NewString("list"): NewList([]BenType{
						NewString("string in list"),
						NewInteger(333923987),
					}),
					*NewString("another key"): NewDictionary(map[String]BenType{
						*NewString("nested dict key"): NewInteger(321),
					}),
				}),
			},
			expected: "5:helloi123ed11:another keyd15:nested dict keyi321ee4:listl14:string in listi333923987eee",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := Encode(buf, tt.data)
			assert.NoError(t, err)
			assert.Equal(t, tt.expected, buf.String())
		})
	}
}

func TestDecode(t *testing.T) {
	r, err := os.Open("testdata/torrent_test.torrent")
	defer r.Close()
	require.NoError(t, err)
	_, err = Decode(r)
	assert.NoError(t, err)
}

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
	r, err := os.Open("../../testdata/debian-12.0.0-amd64-netinst.iso.torrent")
	require.NoError(t, err)
	defer r.Close()
	decoded, err := Decode(r)
	require.NoError(t, err)
	require.IsType(t, &Dictionary{}, decoded)
	dict := decoded.(*Dictionary)
	infoDictDecoded := dict.Get("info")
	require.IsType(t, &Dictionary{}, infoDictDecoded)
	infoDict := infoDictDecoded.(*Dictionary)
	assert.Equal(t, NewString("http://bttracker.debian.org:6969/announce"), dict.Get("announce"))
	assert.Equal(t, NewString("debian-12.0.0-amd64-netinst.iso"), infoDict.Get("name"))
	assert.Equal(t, NewInteger(262144), infoDict.Get("piece length"))
	assert.Len(t, infoDict.Get("pieces").String(), 291510)
}

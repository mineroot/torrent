package bencode

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"strings"
	"testing"
)

func TestDictionary_Encode_Decode(t *testing.T) {
	tests := []struct {
		name    string
		decoded *Dictionary
		bencode string
	}{
		{
			name:    "empty dict",
			decoded: NewDictionary(map[String]BenType{}),
			bencode: "de",
		},
		{
			name: "dict of strings",
			decoded: NewDictionary(map[String]BenType{
				*NewString("cow"):  NewString("moo"),
				*NewString("spam"): NewString("eggs"),
			}),
			bencode: "d3:cow3:moo4:spam4:eggse",
		},
		{
			name: "dict of strings reverse sorted",
			decoded: NewDictionary(map[String]BenType{
				*NewString("spam"): NewString("eggs"),
				*NewString("cow"):  NewString("moo"),
			}),
			bencode: "d3:cow3:moo4:spam4:eggse",
		},
		{
			name: "dict of integers",
			decoded: NewDictionary(map[String]BenType{
				*NewString("key1"): NewInteger(123),
				*NewString("2key"): NewInteger(-321),
			}),
			bencode: "d4:2keyi-321e4:key1i123ee",
		},
		{
			name: "dict of dict",
			decoded: NewDictionary(map[String]BenType{
				*NewString("dictKey"): NewDictionary(map[String]BenType{
					*NewString("nestedDictKey"): NewString("hello"),
				}),
			}),
			bencode: "d7:dictKeyd13:nestedDictKey5:helloee",
		},
		{
			name: "unordered dict of string, list, dict",
			decoded: NewDictionary(map[String]BenType{
				*NewString("key"):        NewString("moo"),
				*NewString("anotherKey"): NewInteger(-500),
				*NewString("dictKey"): NewDictionary(map[String]BenType{
					*NewString("nestedDictKey1"): NewString("hello"),
					*NewString("nestedDictKey2"): NewList([]BenType{NewInteger(123), NewString("test")}),
				}),
			}),
			bencode: "d10:anotherKeyi-500e7:dictKeyd14:nestedDictKey15:hello14:nestedDictKey2li123e4:testee3:key3:mooe",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := &bytes.Buffer{}
			err := tt.decoded.Encode(buf)
			require.NoError(t, err)
			assert.Equal(t, tt.bencode, buf.String())

			dict := NewDictionary(nil)
			err = dict.Decode(strings.NewReader(tt.bencode))
			require.NoError(t, err)
			assert.Equal(t, tt.decoded, dict)
		})
	}
}

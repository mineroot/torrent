package peer

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"

	"github.com/mineroot/torrent/pkg/bitfield"
	"github.com/mineroot/torrent/pkg/download"
)

func TestMessage_Encode(t *testing.T) {
	bf, err := bitfield.FromPayload([]byte{0b11100100, 0b10110000}, 12)
	require.NoError(t, err)
	tests := map[string]struct {
		msg      *Message
		expected []byte
	}{
		"unchoke": {
			msg:      NewUnChoke(),
			expected: []byte{0x0, 0x0, 0x0, 0x1, 0x1},
		},
		"interested": {
			msg:      NewInterested(),
			expected: []byte{0x0, 0x0, 0x0, 0x1, 0x2},
		},
		"bitfield": {
			msg:      NewBitfield(bf),
			expected: []byte{0x0, 0x0, 0x0, 0x3, 0x5, 0b11100100, 0b10110000},
		},
		"request": {
			msg:      NewRequest(download.NewBlock(8, 0, 255)),
			expected: []byte{0x0, 0x0, 0x0, 0xd, 0x6, 0x0, 0x0, 0x0, 0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xFF},
		},
		"piece": {
			msg:      NewPiece(download.NewBlock(8, 0, 3), []byte{0xFF, 0xFA, 0xAA}),
			expected: []byte{0x0, 0x0, 0x0, 0xc, 0x7, 0x0, 0x0, 0x0, 0x8, 0x0, 0x0, 0x0, 0x0, 0xff, 0xfa, 0xaa},
		},
		"cancel": {
			msg:      NewCancel(download.NewBlock(8, 0, 255)),
			expected: []byte{0x0, 0x0, 0x0, 0xd, 0x8, 0x0, 0x0, 0x0, 0x8, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0xFF},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.msg.Encode())
		})
	}
}

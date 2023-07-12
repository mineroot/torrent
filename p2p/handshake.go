package p2p

import (
	"bytes"
	"fmt"
	"io"
)

type handshake struct {
	infoHash Hash
	peerID   PeerID
}

func newHandshake(infoHash Hash, peerId PeerID) *handshake {
	return &handshake{
		infoHash: infoHash,
		peerID:   peerId,
	}
}

// encode <pstrlen><pstr><reserved><info_hash><peer_id>
func (h *handshake) encode() []byte {
	buf := make([]byte, handshakeLen)
	buf[0] = byte(len(pstr))
	curr := 1
	curr += copy(buf[curr:], pstr)
	curr += copy(buf[curr:], make([]byte, 8)) // 8 reserved bytes
	curr += copy(buf[curr:], h.infoHash[:])
	curr += copy(buf[curr:], h.peerID[:])
	return buf
}

func (h *handshake) decode(raw []byte) error {
	if h == nil {
		panic("h must be not nil")
	}
	r := bytes.NewReader(raw)
	// read pstrlen
	if pstrLen, err := r.ReadByte(); err != nil || pstrLen != byte(len(pstr)) {
		return fmt.Errorf("invalid handshake: unable to read pstrlen")
	}
	// read pstr
	buf := make([]byte, len(pstr))
	if _, err := io.ReadFull(r, buf); err != nil || string(buf) != pstr {
		return fmt.Errorf("invalid handshake: unable to read pstr")
	}
	// read reserved 8 bytes
	buf = make([]byte, 8)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("invalid handshake: unable to read reserved bytes")
	}
	// read info_hash
	buf = make([]byte, HashSize)
	if _, err := io.ReadFull(r, buf); err != nil || (!h.infoHash.IsZero() && !bytes.Equal(buf, h.infoHash[:])) {
		return fmt.Errorf("invalid handshake: unable to read info_hash")
	}
	if h.infoHash.IsZero() {
		h.infoHash = (Hash)(buf)
	}
	// read peer_id
	buf = make([]byte, PeerIdSize)
	if _, err := io.ReadFull(r, buf); err != nil {
		return fmt.Errorf("invalid handshake: unable to read peer_id")
	}
	h.peerID = PeerID(buf)

	return nil
}

package torrent

import (
	"crypto/sha1"
	"encoding/hex"
)

const HashSize = sha1.Size

type Hashable interface {
	GetHash() Hash
}

type Hash [HashSize]byte

func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// IsZero despite zero hash is completely valid SHA1, we assume it as nil value to not deal with nil checks,
// we are not so lucky to find real zero hash
func (h Hash) IsZero() bool {
	return h == Hash{}
}

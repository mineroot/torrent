package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"torrent/bencode"
)

type File struct {
	TorrentFileName string
	DownloadDir     string
	Announce        string
	InfoHash        Hash
	PieceHashes     []Hash
	PieceLength     int
	Length          int
	Name            string
}

func Open(torrentFileName, downloadDir string) (*File, error) {
	file, err := os.Open(torrentFileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	torrent := &File{
		TorrentFileName: file.Name(),
		DownloadDir:     downloadDir,
	}
	benType, err := bencode.Decode(file)
	if err != nil {
		return nil, err
	}
	if err = torrent.unmarshal(benType); err != nil {
		return nil, err
	}
	return torrent, nil
}

func (t *File) PiecesCount() int {
	return len(t.PieceHashes)
}

func (t *File) unmarshal(benType bencode.BenType) error {
	if t == nil {
		panic("torrent must be not nil")
	}
	dict, ok := benType.(*bencode.Dictionary)
	if !ok {
		return fmt.Errorf("torrent must be a dictionary")
	}

	announce, ok := dict.Get("announce").(*bencode.String)
	if !ok {
		return fmt.Errorf("announce must be a string")
	}

	infoDict, ok := dict.Get("info").(*bencode.Dictionary)
	if !ok {
		return fmt.Errorf("info must be a dictionary")
	}

	infoEncoded := &bytes.Buffer{}
	if err := infoDict.Encode(infoEncoded); err != nil {
		return fmt.Errorf("unable to encode info")
	}
	infoHash := sha1.Sum(infoEncoded.Bytes())

	name, ok := infoDict.Get("name").(*bencode.String)
	if !ok {
		return fmt.Errorf("name must be a string")
	}

	length, ok := infoDict.Get("length").(*bencode.Integer)
	if !ok {
		return fmt.Errorf("length must be an integer")
	}
	lengthInt := int(length.Value())

	pieceLength, ok := infoDict.Get("piece length").(*bencode.Integer)
	if !ok {
		return fmt.Errorf("piece length must be an integer")
	}
	pieceLengthInt := int(pieceLength.Value())

	pieces, ok := infoDict.Get("pieces").(*bencode.String)
	if !ok {
		return fmt.Errorf("pieces must be bytes")
	}
	piecesBytes := []byte(pieces.Value())
	if len(piecesBytes)%HashSize != 0 {
		return fmt.Errorf("malformed pieses, must be multiple of %d", HashSize)
	}
	piecesCount := len(piecesBytes) / HashSize
	pieceHashes := make([]Hash, piecesCount)
	for i := 0; i < piecesCount; i++ {
		offset := i * HashSize
		pieceHashes[i] = (Hash)(piecesBytes[offset : offset+HashSize])
	}

	t.Announce = announce.Value()
	t.Name = name.Value()
	t.Length = lengthInt
	t.PieceLength = pieceLengthInt
	t.PieceHashes = pieceHashes
	t.InfoHash = infoHash
	return nil
}

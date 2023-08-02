package torrent

import (
	"bytes"
	"crypto/sha1"
	"fmt"
	"os"
	"path"

	"github.com/mineroot/torrent/pkg/bencode"
)

type DownloadFile struct {
	Length int64
	Name   string
}

type File struct {
	TorrentFileName string
	DownloadDir     string
	Announce        string
	InfoHash        Hash
	PieceHashes     []Hash
	PieceLength     int
	DownloadFiles   []DownloadFile
	totalLength     int64
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
		DownloadFiles:   make([]DownloadFile, 0),
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

func (f *File) PiecesCount() int {
	return len(f.PieceHashes)
}

func (f *File) DownloadFilesCount() int {
	return len(f.DownloadFiles)
}

func (f *File) TotalLength() int64 {
	if f.totalLength == 0 {
		for _, file := range f.DownloadFiles {
			f.totalLength += file.Length
		}
	}
	return f.totalLength
}

func (f *File) unmarshal(benType bencode.BenType) error {
	if f == nil {
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

	// filename or dirname depending on mode bellow
	name, ok := infoDict.Get("name").(*bencode.String)
	if !ok {
		return fmt.Errorf("name must be a string")
	}

	if length, ok := infoDict.Get("length").(*bencode.Integer); ok { // single file mode
		f.DownloadFiles = append(f.DownloadFiles, DownloadFile{
			Length: length.Value(),
			Name:   path.Join(f.DownloadDir, name.Value()),
		})
	} else { // multiple file mode
		files, ok := infoDict.Get("files").(*bencode.List)
		if !ok {
			return fmt.Errorf("files list doesn't exist")
		}
		for _, fileBenType := range files.Value() {
			fileDict, ok := fileBenType.(*bencode.Dictionary)
			if !ok {
				return fmt.Errorf("files list item isn't a dict")
			}
			length, ok := fileDict.Get("length").(*bencode.Integer)
			if !ok {
				return fmt.Errorf("file's length must be an integer")
			}
			pathList, ok := fileDict.Get("path").(*bencode.List)
			if !ok {
				return fmt.Errorf("file's path must be a list")
			}
			filePath := f.DownloadDir
			for _, elem := range pathList.Value() {
				elemStr, ok := elem.(*bencode.String)
				if !ok {
					return fmt.Errorf("file's path elem must be a string")
				}
				filePath = path.Join(filePath, elemStr.Value())
			}

			f.DownloadFiles = append(f.DownloadFiles, DownloadFile{
				Length: length.Value(),
				Name:   filePath,
			})
		}
	}
	if len(f.DownloadFiles) == 0 {
		return fmt.Errorf("empty files dict")
	}

	f.Announce = announce.Value()
	f.PieceLength = pieceLengthInt
	f.PieceHashes = pieceHashes
	f.InfoHash = infoHash
	return nil
}

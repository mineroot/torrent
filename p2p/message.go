package p2p

type messageId int

const (
	msgChoke         messageId = 0
	msgUnChoke                 = 1
	msgInterested              = 2
	msgNotInterested           = 3
	msgHave                    = 4
	msgBitfield                = 5
	msgRequest                 = 6
	msgPiece                   = 7
	msgCancel                  = 8
	msgPort                    = 9
)

type Message struct {
	ID      messageId
	Payload []byte
}

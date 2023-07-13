package p2p

import "net/http"

type Announcer interface {
	Do(req *http.Request) (*http.Response, error)
}

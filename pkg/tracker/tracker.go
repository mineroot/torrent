package tracker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/mineroot/torrent/pkg/bencode"

	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/torrent"
)

type Tracker struct {
	httpClient *http.Client
	torrent    *torrent.File
	peerId     peer.ID
	port       uint16
	peersCh    chan<- peer.Peers
	interval   time.Duration
}

func New(torrent *torrent.File, peerId peer.ID, port uint16, peersCh chan<- peer.Peers) *Tracker {
	return &Tracker{
		httpClient: &http.Client{},
		torrent:    torrent,
		peerId:     peerId,
		port:       port,
		peersCh:    peersCh,
	}
}

func (t *Tracker) Run(ctx context.Context) error {
	err := t.handleEvent(ctx, started)
	if err != nil {
		return fmt.Errorf("tracker: %w", err)
	}
	defer t.exit()

	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err = t.handleEvent(ctx, regular)
			if err != nil {
				return fmt.Errorf("tracker: %w", err) // todo: do not return?
			}
		}
	}
}

func (t *Tracker) handleEvent(ctx context.Context, event event) (err error) {
	resp, err := t.trackerRequest(ctx, event)
	if err != nil {
		return
	}
	if event == started {
		t.interval = resp.interval
	}
	t.peersCh <- resp.peers
	return
}

func (t *Tracker) trackerRequest(ctx context.Context, event event) (*response, error) {
	trackerURL, err := t.buildTrackerURL(event)
	if err != nil {
		return nil, fmt.Errorf("unable to buid tracker url: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", trackerURL, nil)
	if err != nil {
		return nil, fmt.Errorf("unable to create new http request: %w", err)
	}
	httpResp, err := t.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("unable to send http request: %w", err)
	}
	defer httpResp.Body.Close()
	if event == stopped {
		// no need to decode response
		return nil, fmt.Errorf("no response")
	}
	b, err := io.ReadAll(httpResp.Body)
	if err != nil {
		return nil, fmt.Errorf("unable to read from body tracker url: %w", err)
	}
	buf := bytes.NewBuffer(b)
	decoded, err := bencode.Decode(buf)
	if err != nil {
		return nil, fmt.Errorf("unable to decode bencode: %w", err)
	}
	resp := newTrackerResponse(t.torrent.InfoHash)
	err = resp.unmarshal(decoded)
	if err != nil {
		return nil, fmt.Errorf("unable unmarchal to response: %w", err)
	}
	return resp, nil
}

func (t *Tracker) exit() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, _ = t.trackerRequest(ctx, stopped)
}

func (t *Tracker) buildTrackerURL(event event) (string, error) {
	base, err := url.Parse(t.torrent.Announce)
	if err != nil {
		return "", err
	}
	peerId := t.peerId.PeerId()
	params := url.Values{
		"info_hash":  []string{string(t.torrent.InfoHash[:])},
		"peer_id":    []string{string(peerId[:])},
		"port":       []string{strconv.Itoa(int(t.port))},
		"uploaded":   []string{"0"},
		"downloaded": []string{"0"},
		"compact":    []string{"1"},
		"left":       []string{strconv.Itoa(t.torrent.Length)},
		"event":      []string{string(event)},
		"numwant":    []string{"100"},
	}
	base.RawQuery = params.Encode()
	return base.String(), nil
}

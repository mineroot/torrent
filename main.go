package main

import (
	"context"
	"errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"torrent/p2p"
)

func main() {
	l := log.Output(zerolog.ConsoleWriter{Out: os.Stderr}).With().Caller().Logger()
	file, err := os.Open("testdata/debian-12.0.0-amd64-netinst.iso.torrent")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	defer file.Close()
	torrent, err := p2p.Open(file)
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	storage := p2p.NewStorage()
	storage.Set(torrent.InfoHash, torrent)
	client := p2p.NewClient(storage)

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)

	errCh := make(chan error, 1)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		err = client.Run(ctx)
		if errors.Is(err, context.Canceled) {
			return
		}
		errCh <- err
	}()
	select {
	case <-exit:
		cancel()
		wg.Wait()
	case err = <-errCh:
		if err != nil {
			log.Printf("error: %s", err)
		}
	}
}

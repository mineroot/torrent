package main

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"sync"
	"torrent/p2p"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
	"torrent/ui"
)

const listenPort uint16 = 6881

func main() {
	logFile, err := os.OpenFile("app.log", os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		fmt.Println("unable to create log.log file")
		return
	}
	defer logFile.Close()

	l := log.Output(zerolog.ConsoleWriter{Out: logFile}).With().Caller().Logger().Level(zerolog.InfoLevel)
	s := storage.NewStorage()

	t1, err := torrent.Open("testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	err = s.Set(t1.InfoHash, t1)
	if err != nil {
		l.Fatal().Err(err).Send()
	}

	t2, err := torrent.Open("/home/mineroot/Desktop/debian-11.5.0-amd64-netinst.iso.torrent", "")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	err = s.Set(t2.InfoHash, t2)
	if err != nil {
		l.Fatal().Err(err).Send()
	}

	client := p2p.NewClient(p2p.PeerID([]byte("-GO0001-random_bytes")), listenPort, s, &http.Client{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = client.Run(ctx)
	}()

	app := ui.CreateApp(s, client.ProgressSpeed(), client.ProgressPieces())
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			cancel()
			app.Stop()
			fmt.Print("Shouting down...")
		}
		return event
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := app.Run(); err != nil {
			panic(err)
		}
	}()

	wg.Wait()
	fmt.Println(" done")
}

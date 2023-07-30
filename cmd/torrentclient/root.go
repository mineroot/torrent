package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/afero"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"os"
	"path"
	"path/filepath"

	"github.com/mineroot/torrent/pkg"
	"github.com/mineroot/torrent/pkg/peer"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
	"github.com/mineroot/torrent/ui"
)

const listenPort uint16 = 6881 // TODO: remove. use range of ports to find available

var (
	torrents     []string
	destinations []string
)

var rootCmd = &cobra.Command{
	DisableFlagsInUseLine: true,
	Version:               "0.1",
	Use:                   "torrent-client (file.torrent destination_dir)...",
	Example:               "  torrent-client debian.torrent ~/Downloads another.torrent /destination",
	Short:                 "BitTorrent Client written in Go",
	RunE:                  run,
	Args: func(cmd *cobra.Command, args []string) error {
		if len(args) < 2 {
			return fmt.Errorf("requires at least 2 args, only received %d", len(args))
		}
		if len(args)%2 != 0 {
			return fmt.Errorf("expected one or more pairs of (file.torrent destination_dir), received %d args", len(args))
		}

		for i, arg := range args {
			isTorrent := i%2 == 0
			if isTorrent {
				torrents = append(torrents, arg)
			} else {
				destinations = append(destinations, arg)
			}
		}
		return nil
	},
}

func run(*cobra.Command, []string) error {
	logFile, err := openLogFile()
	if err != nil {
		return err
	}
	defer logFile.Close()

	l := log.Output(zerolog.ConsoleWriter{Out: logFile}).With().Caller().Logger().Level(zerolog.InfoLevel)

	s := storage.NewStorage(afero.NewOsFs())
	for i := 0; i < len(torrents); i++ {
		torrentPath, err := filepath.Abs(torrents[i])
		if err != nil {
			return fmt.Errorf("unable to get an absolute path of torrent file: %w", err)
		}
		destinationPath, err := filepath.Abs(destinations[i])
		if err != nil {
			return fmt.Errorf("unable to get an absolute path of destination: %w", err)
		}
		t, err := torrent.Open(torrentPath, destinationPath)
		if err != nil {
			return fmt.Errorf("unable to open torrent file: %w", err)
		}
		if err = s.Set(t); err != nil {
			return fmt.Errorf("unable to add torrent to storage: %w", err)
		}
	}

	client := pkg.NewClient(peer.IDFromString("-GO0001-random_bytes"), listenPort, s)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)

	app := ui.CreateApp(s, client.ProgressSpeed(), client.ProgressPieces())
	app.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			cancel()
			app.Stop()
		}
		return event
	})

	g, ctx := errgroup.WithContext(ctx)
	g.Go(app.Run)
	g.Go(func() error {
		defer app.Stop()
		return client.Run(ctx)
	})
	if err = g.Wait(); errors.Is(err, context.Canceled) {
		return nil
	}
	return err
}

func openLogFile() (*os.File, error) {
	const logDir, logFile = ".torrent-client", "app.log"
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("unable get user's home directory: %w", err)
	}
	logDirPath := path.Join(homeDir, logDir)
	if _, err = os.Stat(logDirPath); os.IsNotExist(err) {
		if err = os.Mkdir(logDirPath, 0755); err != nil {
			return nil, fmt.Errorf("unable to create log directory: %w", err)
		}
	} else if err != nil {
		return nil, fmt.Errorf("unable to check log directory: %w", err)
	}
	logFilePath := path.Join(logDirPath, logFile)

	logFileFd, err := os.OpenFile(logFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0664)
	if err != nil {
		return nil, fmt.Errorf("unable to open log file: %w", err)
	}
	return logFileFd, nil
}

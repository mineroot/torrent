package main

import (
	"context"
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
	"strconv"
	"sync"
	"torrent/p2p"
	"torrent/p2p/storage"
	"torrent/p2p/torrent"
	"torrent/utils"
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
	t1, err := torrent.Open("testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	t2, err := torrent.Open("/home/mineroot/Desktop/debian-11.5.0-amd64-netinst.iso.torrent", "")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	s := storage.NewStorage()
	err = s.Set(t1.InfoHash, t1)
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

	ui := createUi(s, client.ProgressSpeed())
	ui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			cancel()
			ui.Stop()
			fmt.Print("Shouting down...")
		}
		return event
	})
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := ui.Run(); err != nil {
			panic(err)
		}
	}()

	wg.Wait()
	fmt.Println(" done")
}

func createUi(s storage.Reader, progressSpeedCh <-chan *p2p.ProgressSpeed) *tview.Application {
	// TODO refactor
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true)

	headers := []string{
		"Name",
		"Size",
		"Pieces (total)",
		"Pieces (downloaded)",
		"Download speed",
	}
	for col := 0; col < len(headers); col++ {
		table.SetCell(0, col, tview.NewTableCell(headers[col]).SetTextColor(tcell.ColorYellow))
	}

	row := 1
	rowByHash := make(map[torrent.Hash]int)
	for file := range s.Iterator() {
		rowByHash[file.InfoHash] = row
		table.SetCell(row, 0, tview.NewTableCell(file.TorrentFileName))
		table.SetCell(row, 1, tview.NewTableCell(utils.FormatBytes(uint(file.Length))))
		table.SetCell(row, 2, tview.NewTableCell(strconv.Itoa(file.PiecesCount())))
		table.SetCell(row, 3, tview.NewTableCell("0"))
		table.SetCell(row, 4, tview.NewTableCell("0 B/s"))
		row++
	}
	go func() {
		for progressSpeed := range progressSpeedCh {
			row := rowByHash[progressSpeed.Hash]
			cell := table.GetCell(row, 4)
			app.QueueUpdateDraw(func() {
				speed := fmt.Sprintf("%s/s", utils.FormatBytes(uint(progressSpeed.Speed)))
				cell.SetText(speed)
			})
		}
	}()
	//go func() {
	//	for {
	//		progress := <-progressCh
	//		row, ok := rowByHash[progress.Hash()]
	//		if !ok {
	//			continue
	//		}
	//		switch progress.ID() {
	//		case p2p.ProgressPieceDownloaded:
	//			cell := table.GetCell(row, 3)
	//			downloadedCount, _ := strconv.Atoi(cell.Text)
	//			downloadedCount++
	//			app.QueueUpdateDraw(func() {
	//				cell.SetText(strconv.Itoa(downloadedCount))
	//			})
	//		case p2p.ProgressConnRead:
	//
	//		}
	//	}
	//}()

	table.Select(0, 0).SetFixed(1, 1).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			table.SetSelectable(true, false)
		}
	}).SetSelectedFunc(func(row int, column int) {
		table.GetCell(row, column).SetTextColor(tcell.ColorRed)
		table.SetSelectable(false, false)
	})
	return app.SetRoot(table, true).SetFocus(table)
}

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

	l := log.Output(zerolog.ConsoleWriter{Out: logFile}).With().Caller().Logger().Level(zerolog.InfoLevel)
	t, err := torrent.Open("testdata/debian-12.0.0-amd64-netinst.iso.torrent", "")
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	s := storage.NewStorage()
	err = s.Set(t.InfoHash, t)
	if err != nil {
		l.Fatal().Err(err).Send()
	}
	client := p2p.NewClient(p2p.PeerID([]byte("-GO0001-random_bytes")), listenPort, s, &http.Client{})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx = l.WithContext(ctx)

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_ = client.Run(ctx)
	}()

	ui := createUi(s, client.Progress())
	ui.SetInputCapture(func(event *tcell.EventKey) *tcell.EventKey {
		if event.Key() == tcell.KeyCtrlC || event.Key() == tcell.KeyEscape {
			cancel()
			ui.Stop()
			fmt.Print("Shouting down...")
		}
		return event
	})
	go func() {
		defer wg.Done()
		if err := ui.Run(); err != nil {
			panic(err)
		}
	}()

	wg.Wait()
	fmt.Println(" done")
}

func createUi(s storage.Reader, progressCh <-chan *p2p.Progress) *tview.Application {
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true)

	headers := []string{
		"Name",
		"Size",
		"Pieces (total)",
		"Pieces (downloaded)",
	}
	for col := 0; col < len(headers); col++ {
		table.SetCell(0, col, tview.NewTableCell(headers[col]).SetTextColor(tcell.ColorYellow))
	}

	row := 1
	for file := range s.Iterator() {
		table.SetCell(row, 0, tview.NewTableCell(file.TorrentFileName))
		table.SetCell(row, 1, tview.NewTableCell(utils.FormatBytes(uint(file.Length))))
		table.SetCell(row, 2, tview.NewTableCell(strconv.Itoa(file.PiecesCount())))
		table.SetCell(row, 3, tview.NewTableCell("0"))
		row++
	}
	go func() {
		for {
			<-progressCh // TODO
			cell := table.GetCell(1, 3)
			downloadedCount, _ := strconv.Atoi(cell.Text)
			downloadedCount++
			app.QueueUpdateDraw(func() {
				cell.SetText(strconv.Itoa(downloadedCount))
			})
		}
	}()

	table.Select(0, 0).SetFixed(1, 1).SetDoneFunc(func(key tcell.Key) {
		if key == tcell.KeyEnter {
			table.SetSelectable(true, false)
		}
	}).SetSelectedFunc(func(row int, column int) {
		table.GetCell(row, column).SetTextColor(tcell.ColorRed)
		table.SetSelectable(false, false)
	})

	//lorem := strings.Split("Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet. Lorem ipsum dolor sit amet, consetetur sadipscing elitr, sed diam nonumy eirmod tempor invidunt ut labore et dolore magna aliquyam erat, sed diam voluptua. At vero eos et accusam et justo duo dolores et ea rebum. Stet clita kasd gubergren, no sea takimata sanctus est Lorem ipsum dolor sit amet.", " ")
	//cols, rows := 10, 40
	//word := 0
	//for r := 0; r < rows; r++ {
	//	for c := 0; c < cols; c++ {
	//		color := tcell.ColorWhite
	//		if c < 1 || r < 1 {
	//			color = tcell.ColorYellow
	//		}
	//		table.SetCell(r, c,
	//			tview.NewTableCell(lorem[word]).
	//				SetTextColor(color).
	//				SetAlign(tview.AlignCenter))
	//		word = (word + 1) % len(lorem)
	//	}
	//}
	//table.Select(0, 0).SetFixed(1, 1).SetDoneFunc(func(key tcell.Key) {
	//	if key == tcell.KeyEnter {
	//		table.SetSelectable(true, false)
	//	}
	//}).SetSelectedFunc(func(row int, column int) {
	//	table.GetCell(row, column).SetTextColor(tcell.ColorRed)
	//	table.SetSelectable(false, false)
	//})
	return app.SetRoot(table, true).SetFocus(table)
}

package ui

import (
	"fmt"
	"github.com/gdamore/tcell/v2"
	"github.com/rivo/tview"
	"strconv"
	"strings"
	"time"

	"github.com/mineroot/torrent/pkg/event"
	"github.com/mineroot/torrent/pkg/storage"
	"github.com/mineroot/torrent/pkg/torrent"
	"github.com/mineroot/torrent/utils"
)

func CreateApp(
	s storage.Reader,
	progressSpeedCh <-chan *event.ProgressSpeed,
	progressPieces <-chan *event.ProgressPieceDownloaded,
) *tview.Application {
	app := tview.NewApplication()
	table := tview.NewTable().
		SetBorders(true)

	headers := []string{
		"Name",
		"Size",
		"Pieces (total)",
		"Pieces (downloaded)",
		"Download speed",
		"Progress",
	}
	for col := 0; col < len(headers); col++ {
		table.SetCell(0, col, tview.NewTableCell(headers[col]).SetTextColor(tcell.ColorYellow))
	}

	row := 1
	rowByHash := make(map[torrent.Hash]int)
	for td := range s.Iterator() {
		rowByHash[td.InfoHash()] = row
		table.SetCell(row, 0, tview.NewTableCell(td.Torrent().TorrentFileName))
		table.SetCell(row, 1, tview.NewTableCell(utils.FormatBytes(uint(td.Torrent().TotalLength()))))
		table.SetCell(row, 2, tview.NewTableCell(strconv.Itoa(td.Torrent().PiecesCount())))
		table.SetCell(row, 3, tview.NewTableCell("0"))
		table.SetCell(row, 4, tview.NewTableCell("0 B/s"))
		table.SetCell(row, 5, tview.NewTableCell(strings.Repeat("□", 20)))
		row++
	}
	go func() {
		for progressSpeed := range progressSpeedCh {
			row := rowByHash[progressSpeed.Hash]
			cell := table.GetCell(row, 4)
			app.QueueUpdate(func() {
				speed := fmt.Sprintf("%s/s", utils.FormatBytes(uint(progressSpeed.Speed)))
				cell.SetText(speed)
			})
		}
	}()
	go func() {
		for {
			progressPiece := <-progressPieces
			row := rowByHash[progressPiece.Hash]
			app.QueueUpdate(func() {
				table.GetCell(row, 3).SetText(strconv.Itoa(progressPiece.DownloadedCount))
			})

			td := s.Get(progressPiece.Hash)
			progressBarMaxWidth := 20
			progressBarMax := td.Torrent().PiecesCount()
			progressBarCurrent := progressPiece.DownloadedCount
			progressBarCurrentWidth := progressBarMaxWidth * progressBarCurrent / progressBarMax

			app.QueueUpdate(func() {
				progressTxt := fmt.Sprintf("%s%s", strings.Repeat("■", progressBarCurrentWidth), strings.Repeat("□", progressBarMaxWidth-progressBarCurrentWidth))
				table.GetCell(row, 5).SetText(progressTxt)
			})
		}
	}()
	go func() {
		for {
			<-time.Tick(time.Second)
			app.Draw()
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
	return app.SetRoot(table, true).SetFocus(table)
}

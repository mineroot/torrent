package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mineroot/torrent/pkg/tracker"
)

func main() {
	ft := tracker.NewFakeTracker(":8080")
	go func() {
		fmt.Println(ft.ListenAndServe())
	}()

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, os.Interrupt, syscall.SIGTERM)
	<-exit
	_ = ft.Shutdown(context.TODO())
}

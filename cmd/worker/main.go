package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

func main() {
	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	g, gCtx := errgroup.WithContext(mainCtx)

	g.Go(func() error {
		for {
			select {
			case <-time.After(1 * time.Second):
				fmt.Println(1)
			case <-gCtx.Done():
				return nil
			}
		}
	})

	g.Go(func() error {
		for {
			select {
			case <-time.After(1 * time.Second):
				fmt.Println(2)
			case <-gCtx.Done():
				return nil
			}
		}
	})

	if err := g.Wait(); err != nil {
		fmt.Sprint("Error group", err)
	}

	fmt.Println(`Done main`)
}

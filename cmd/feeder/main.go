package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	defer stop()
	g, gCtx := errgroup.WithContext(ctx)

	cfg, envErr := env.Read("")
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log := logger.New(&cfg.AppConfig)

	transport := &http.Transport{
		MaxIdleConns:          1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	g.Go(func() error {
		for {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			case <-ticker.C:
				resp, err := getLatestBlock[EthBlock](gCtx, cfg.AppConfig.JsonRpcURL, httpClient)
				if err != nil {
					fmt.Println("GetLatestBlock error:", err.Error())
				} else {
					value, _ := strconv.ParseInt(resp.Result.Number, 0, 64)

					fmt.Println(value)
				}
			}
		}
	})

	g.Go(func() error {
		<-gCtx.Done()
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("g.Wait error:", err.Error())
	}

	fmt.Printf(`Feeder done`)
}

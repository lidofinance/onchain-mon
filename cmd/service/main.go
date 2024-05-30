package main

import (
	"context"
	"fmt"
	"github.com/go-chi/chi/v5"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	server "github.com/lidofinance/finding-forwarder/internal/app/http_server"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, envErr := env.Read()
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log, logErr := logger.New(&cfg.AppConfig)
	if logErr != nil {
		fmt.Println("Logger error:", logErr.Error())
		return
	}

	log.Info(fmt.Sprintf(`started %s application`, cfg.AppConfig.Name))

	r := chi.NewRouter()
	metrics := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.Name, cfg.AppConfig.Env)

	usecase := server.Usecase()

	app := server.New(log, metrics, usecase)

	app.Metrics.BuildInfo.Inc()
	app.RegisterRoutes(r)

	g, gCtx := errgroup.WithContext(ctx)

	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)
	// someDaemon(gCtx, g)

	if err := g.Wait(); err != nil {
		log.Error(err)
	}

	fmt.Println(`Main done`)
}

func someDaemon(gCtx context.Context, g *errgroup.Group) {
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
}

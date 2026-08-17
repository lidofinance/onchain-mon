package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/onchain-mon/internal/app/feeder"
	"github.com/lidofinance/onchain-mon/internal/app/server"
	"github.com/lidofinance/onchain-mon/internal/connectors/logger"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	nc "github.com/lidofinance/onchain-mon/internal/connectors/nats"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain"
)

func main() {
	// run returns the error so deferred cleanup still happens before os.Exit.
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()
	g, gCtx := errgroup.WithContext(ctx)

	cfg, envErr := env.Read("")
	if envErr != nil {
		return fmt.Errorf("read env: %w", envErr)
	}

	log, sentryClient, logErr := logger.New(&cfg.AppConfig)
	if logErr != nil {
		return fmt.Errorf("create logger: %w", logErr)
	}
	if sentryClient != nil {
		defer sentryClient.Flush(2 * time.Second)
	}

	natsClient, natsErr := nc.New(&cfg.AppConfig, log)
	if natsErr != nil {
		return fmt.Errorf("connect to nats: %w", natsErr)
	}
	defer natsClient.Close()
	log.Info("Nats connected")

	js, jetStreamErr := jetstream.New(natsClient)
	if jetStreamErr != nil {
		return fmt.Errorf("connect to jetstream: %w", jetStreamErr)
	}
	log.Info("Nats jetStream connected")

	r := chi.NewRouter()
	metricsStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	transport := &http.Transport{
		MaxIdleConns:          10,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	chainSrv := chain.NewChain(cfg.AppConfig.JsonRpcURL, httpClient, metricsStore)
	app := server.New(&cfg.AppConfig, log, metricsStore, js, natsClient)

	app.Metrics.BuildInfo.Inc()

	feederWrk := feeder.New(log, chainSrv, js, metricsStore, cfg.AppConfig.BlockTopic)
	feederWrk.Run(gCtx, g)

	app.RegisterWorkerRoutes(r)
	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	log.Info("Started feeder")

	if err := g.Wait(); err != nil {
		return fmt.Errorf("%s stopped: %w", cfg.AppConfig.Name, err)
	}

	log.Info("Main done feeder")

	return nil
}

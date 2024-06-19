package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	nc "github.com/lidofinance/finding-forwarder/internal/connectors/nats"

	"github.com/lidofinance/finding-forwarder/internal/app/server"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
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

	natsClient, natsErr := nc.New(&cfg.AppConfig, log)
	if natsErr != nil {
		fmt.Println("Could not connect to nats error:", natsErr.Error())
		return
	}
	defer natsClient.Close()

	js, jetStreamErr := jetstream.New(natsClient)
	if jetStreamErr != nil {
		fmt.Println("Could not connect to jetStream error:", jetStreamErr.Error())
		return
	}

	log.Info(fmt.Sprintf(`started %s`, cfg.AppConfig.Name))

	r := chi.NewRouter()
	promStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	app := server.New(&cfg.AppConfig, log, promStore, js, natsClient)

	app.Metrics.BuildInfo.Inc()
	app.RegisterHTTPRoutes(r)

	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	fmt.Println(`Main done`)
}

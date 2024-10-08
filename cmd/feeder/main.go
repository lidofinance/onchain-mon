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

	"github.com/lidofinance/finding-forwarder/internal/app/feeder"
	"github.com/lidofinance/finding-forwarder/internal/app/server"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	nc "github.com/lidofinance/finding-forwarder/internal/connectors/nats"
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
		log.Error(fmt.Sprintf(`Could not connect to nats error: %v`, natsErr))
		return
	}
	defer natsClient.Close()
	log.Info("Nats connected")

	js, jetStreamErr := jetstream.New(natsClient)
	if jetStreamErr != nil {
		fmt.Println("Could not connect to jetStream error:", jetStreamErr.Error())
		return
	}
	log.Info("Nats jetStream connected")

	r := chi.NewRouter()
	metricsStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	services := server.NewServices(&cfg.AppConfig, metricsStore)
	app := server.New(&cfg.AppConfig, log, metricsStore, js, natsClient)

	app.Metrics.BuildInfo.Inc()

	feederWrk := feeder.New(log, services.ChainSrv, js, metricsStore, cfg.AppConfig.BlockTopic)
	feederWrk.Run(gCtx, g)

	app.RegisterWorkerRoutes(r)
	app.RegisterInfraRoutes(r)
	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	log.Info(fmt.Sprintf(`Started %s feeder`, cfg.AppConfig.Name))

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	fmt.Println(`Main done`)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/finding-forwarder/internal/app/server"
	"github.com/lidofinance/finding-forwarder/internal/app/worker"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	nc "github.com/lidofinance/finding-forwarder/internal/connectors/nats"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/utils/registry"
	"github.com/lidofinance/finding-forwarder/internal/utils/registry/teams"
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

	log.Info(fmt.Sprintf(`started %s worker`, cfg.AppConfig.Name))

	r := chi.NewRouter()
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	services := server.NewServices(&cfg.AppConfig, metricsStore)
	app := server.New(&cfg.AppConfig, log, metricsStore, js, natsClient)

	app.Metrics.BuildInfo.Inc()
	app.RegisterWorkerRoutes(r)

	_ = js.DeleteConsumer(ctx, cfg.AppConfig.NatsStreamName, `Dicorder`)
	_ = js.DeleteConsumer(ctx, cfg.AppConfig.NatsStreamName, `Telegramer`)
	_ = js.DeleteConsumer(ctx, cfg.AppConfig.NatsStreamName, `OpsGeniaer`)
	_ = js.DeleteStream(ctx, cfg.AppConfig.NatsStreamName)

	protocolSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.NatsStreamName, teams.Protocol)
	fallbackSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.NatsStreamName, registry.FallBackTeam)

	commonStreamName := fmt.Sprintf(`%s_STREAM`, cfg.AppConfig.NatsStreamName)

	stream, createStreamErr := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:    commonStreamName,
		Discard: jetstream.DiscardOld,
		MaxAge:  10 * time.Minute,
		Subjects: []string{
			protocolSubject,
			fallbackSubject,
		},
	})

	if createStreamErr != nil && !errors.Is(createStreamErr, nats.ErrStreamNameAlreadyInUse) {
		fmt.Printf("could not create %s stream error: %v\n", commonStreamName, createStreamErr)
		return
	}

	protocolWorker := worker.NewWorker(
		protocolSubject,
		stream, log, metricsStore,
		worker.WithTelegram(services.Telegram, `LidoDbgAlerts`),
		worker.WithDiscord(services.Discord, `DebugDiscord`),
		worker.WithOpsGenia(services.OpsGenia, `BlackBoxOpsGenia`),
	)

	fallbackWorker := worker.NewWorker(
		fallbackSubject,
		stream, log, metricsStore,
		worker.WithTelegram(services.Telegram, `fLidoDbgAlerts`),
		worker.WithDiscord(services.Discord, `fDebugDiscord`),
		worker.WithOpsGenia(services.OpsGenia, `fBlackBoxOpsGenia`),
	)

	if wrkErr := protocolWorker.Run(gCtx, g); wrkErr != nil {
		fmt.Println("Could not start protocolWorker error:", wrkErr.Error())
		return
	}

	if wrkErr := fallbackWorker.Run(gCtx, g); wrkErr != nil {
		fmt.Println("Could not start fallbackWorker error:", wrkErr.Error())
		return
	}

	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	fmt.Println(`Main done`)
}

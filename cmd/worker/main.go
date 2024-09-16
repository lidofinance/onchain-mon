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

	"github.com/lidofinance/finding-forwarder/internal/app/feeder"
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
	metricsStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	services := server.NewServices(&cfg.AppConfig, metricsStore)
	app := server.New(&cfg.AppConfig, log, metricsStore, js, natsClient)

	app.Metrics.BuildInfo.Inc()
	app.RegisterWorkerRoutes(r)

	protocolNatsSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.FindingTopic, teams.Protocol)

	protocolFortaSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.FortaAlertsTopic, teams.Protocol)
	fallbackFortaSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.FortaAlertsTopic, registry.FallBackTeam)

	natsStreamName := `NatsStream`
	fortaStreamName := `FortaStream`

	natStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:    natsStreamName,
		Discard: jetstream.DiscardOld,
		MaxAge:  10 * time.Minute,
		Subjects: []string{
			protocolNatsSubject,
		},
	})

	fortaStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:    fortaStreamName,
		Discard: jetstream.DiscardOld,
		MaxAge:  10 * time.Minute,
		Subjects: []string{
			protocolFortaSubject,
			fallbackFortaSubject,
		},
	})

	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		fmt.Printf("could not create %s stream error: %v\n", natsStreamName, err)
		return
	}

	const (
		Telegram = `Telegram`
		Discord  = `Discord`
		OpsGenie = `OpsGenie`
	)

	protocolWorker := worker.NewFindingWorker(
		protocolNatsSubject,
		natStream, log, metricsStore,
		worker.WithFindingConsumer(services.OnChainAlertsTelegram, `OnChainAlerts_Telegram_Consumer`, registry.OnChainAlerts, Telegram),
		worker.WithFindingConsumer(services.OnChainUpdatesTelegram, `OnChainUpdates_Telegram_Consumer`, registry.OnChainUpdates, Telegram),
		worker.WithFindingConsumer(services.ErrorsTelegram, `Protocol_Errors_Telegram_Consumer`, registry.OnChainErrors, Telegram),
		worker.WithFindingConsumer(services.Discord, `Protocol_Discord_Consumer`, registry.FallBackAlerts, Discord),
		worker.WithFindingConsumer(services.OpsGenia, `Protocol_OpGenia_Consumer`, registry.OnChainAlerts, OpsGenie),
	)

	protocolFortaWorker := worker.NewAlertWorker(
		protocolFortaSubject,
		fortaStream, log, metricsStore,
		worker.WithAlertConsumer(services.OnChainAlertsTelegram, `Forta_OnChainAlerts_Telegram_Consumer`, registry.AlertOnChainAlerts, Telegram),
		worker.WithAlertConsumer(services.OnChainUpdatesTelegram, `Forta_OnChainUpdates_Telegram_Consumer`, registry.AlertOnChainUpdates, Telegram),
		worker.WithAlertConsumer(services.ErrorsTelegram, `Forta_Protocol_Errors_Telegram_Consumer`, registry.AlertOnChainErrors, Telegram),
		worker.WithAlertConsumer(services.Discord, `Forta_Protocol_Discord_Consumer`, registry.AlertFallBackAlerts, Discord),
		worker.WithAlertConsumer(services.OpsGenia, `Forta_Protocol_OpGenia_Consumer`, registry.AlertOnChainAlerts, OpsGenie),
	)

	fallbackFortaWorker := worker.NewAlertWorker(
		fallbackFortaSubject,
		fortaStream, log, metricsStore,
		worker.WithAlertConsumer(services.Discord, `FortaFallback_Discord_Consumer`, registry.AlertFallBackAlerts, Discord),
	)

	feederWrk := feeder.New(log, services.ChainSrv, js, metricsStore, cfg.AppConfig.BlockTopic)

	// Listen findings from Nats
	if err := protocolWorker.Run(gCtx, g); err != nil {
		fmt.Println("Could not start protocolWorker error:", err.Error())
		return
	}

	// Listen alerts from Forta group software
	if err := protocolFortaWorker.Run(gCtx, g); err != nil {
		fmt.Println("Could not start protocolFortaWorker error:", err.Error())
		return
	}

	if wrkErr := fallbackFortaWorker.Run(gCtx, g); wrkErr != nil {
		fmt.Println("Could not start fallbackFortaWorker error:", wrkErr.Error())
		return
	}

	feederWrk.Run(gCtx, g)

	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	fmt.Println(`Main done`)
}

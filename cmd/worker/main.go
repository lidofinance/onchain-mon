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
	"github.com/hashicorp/golang-lru/v2/expirable"
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
	"github.com/lidofinance/finding-forwarder/internal/connectors/redis"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/utils/registry"
	"github.com/lidofinance/finding-forwarder/internal/utils/registry/teams"
)

const maxMsgSize = 3 * 1024 * 1024 // 3 Mb

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

	rds, err := redis.NewRedisClient(gCtx, cfg.AppConfig.RedisURL, log)
	if err != nil {
		log.Error(fmt.Sprintf(`create redis client error: %v`, err))
		return
	}
	defer rds.Close()

	natsClient, natsErr := nc.New(&cfg.AppConfig, log)
	if natsErr != nil {
		log.Error(fmt.Sprintf(`Could not connect to nats error: %v`, err))
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
	app.RegisterWorkerRoutes(r)

	protocolNatsSubject := fmt.Sprintf(`%s.%s`, cfg.AppConfig.FindingTopic, teams.Protocol)

	natsStreamName := `NatsStream`

	natStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:    natsStreamName,
		Discard: jetstream.DiscardOld,
		MaxAge:  10 * time.Minute,
		Subjects: []string{
			protocolNatsSubject,
		},
		MaxMsgSize: maxMsgSize,
	})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		fmt.Printf("could not create %s stream error: %v\n", natsStreamName, err)
		return
	}
	log.Info(fmt.Sprintf("%s jetStream createdOrUpdated", natsStreamName))

	const (
		Telegram = `Telegram`
		Discord  = `Discord`
		OpsGenie = `OpsGenie`
	)

	const LruSize = 125
	cache := expirable.NewLRU[string, uint](LruSize, nil, time.Minute*10)
	protocolWorker := worker.NewFindingWorker(
		log, metricsStore, cache,
		rds, natStream,
		protocolNatsSubject, cfg.AppConfig.QuorumSize,
		worker.WithFindingConsumer(services.OnChainAlertsTelegram, `OnChainAlerts_Telegram_Consumer`, registry.OnChainAlerts, Telegram),
		worker.WithFindingConsumer(services.OnChainUpdatesTelegram, `OnChainUpdates_Telegram_Consumer`, registry.OnChainUpdates, Telegram),
		worker.WithFindingConsumer(services.ErrorsTelegram, `Protocol_Errors_Telegram_Consumer`, registry.OnChainErrors, Telegram),
		worker.WithFindingConsumer(services.Discord, `Protocol_Discord_Consumer`, registry.FallBackAlerts, Discord),
		worker.WithFindingConsumer(services.OpsGenia, `Protocol_OpGenia_Consumer`, registry.OnChainAlerts, OpsGenie),
	)

	// Listen findings from Nats
	if err := protocolWorker.Run(gCtx, g); err != nil {
		fmt.Println("Could not start protocolWorker error:", err.Error())
		return
	}

	feederWrk := feeder.New(log, services.ChainSrv, js, metricsStore, cfg.AppConfig.BlockTopic)
	feederWrk.Run(gCtx, g)

	app.RegisterInfraRoutes(r)
	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	log.Info(fmt.Sprintf(`Started %s worker`, cfg.AppConfig.Name))

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	fmt.Println(`Main done`)
}

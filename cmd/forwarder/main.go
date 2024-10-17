package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/onchain-mon/internal/app/forwarder"
	"github.com/lidofinance/onchain-mon/internal/app/server"
	"github.com/lidofinance/onchain-mon/internal/connectors/logger"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	nc "github.com/lidofinance/onchain-mon/internal/connectors/nats"
	"github.com/lidofinance/onchain-mon/internal/connectors/redis"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
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

	notificationConfig, err := env.ReadNotificationConfig(cfg.AppConfig.Env)
	if err != nil {
		log.Error(fmt.Sprintf("Error loading consumer's config: %v", err))
		return
	}

	rds, err := redis.NewRedisClient(gCtx, cfg.AppConfig.RedisURL, log)
	if err != nil {
		log.Error(fmt.Sprintf(`create redis client error: %v`, err))
		return
	}
	defer rds.Close()

	natsClient, natsErr := nc.New(&cfg.AppConfig, log)
	if natsErr != nil {
		log.Error(fmt.Sprintf(`Could not connect to nats error: %v`, natsErr))
		return
	}
	defer natsClient.Close()
	log.Info("Nats connected")

	js, jetStreamErr := jetstream.New(natsClient)
	if jetStreamErr != nil {
		log.Error(fmt.Sprintf(`Could not connect to jetStream error: %v`, jetStreamErr))
		return
	}
	log.Info("Nats jetStream connected")

	r := chi.NewRouter()
	metricsStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	app := server.New(&cfg.AppConfig, notificationConfig, log, metricsStore, js, natsClient)

	natsStreamName := `NatsStream`
	natStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       natsStreamName,
		Discard:    jetstream.DiscardOld,
		MaxAge:     10 * time.Minute,
		Subjects:   env.CollectNatsSubjects(notificationConfig),
		MaxMsgSize: maxMsgSize,
	})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		fmt.Printf("could not create %s stream error: %v\n", natsStreamName, err)
		return
	}
	log.Info(fmt.Sprintf("%s jetStream createdOrUpdated", natsStreamName))

	transport := &http.Transport{
		MaxIdleConns:          30,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	notificationChannels, err := env.NewNotificationChannels(log, notificationConfig, httpClient, metricsStore, cfg.AppConfig.Source)
	if err != nil {
		log.Error(fmt.Sprintf("Could not init notification channels: %v", err))
		return
	}

	consumers, err := consumer.NewConsumers(
		log,
		metricsStore,
		rds,
		consumer.NewRepo(rds, cfg.AppConfig.QuorumSize),
		cfg.AppConfig.QuorumSize,
		notificationConfig,
		notificationChannels,
	)
	if err != nil {
		log.Error(fmt.Sprintf("Could not init consumers: %v", err))
		return
	}

	worker := forwarder.New(consumers, natStream, log)
	if err := worker.Run(gCtx, g); err != nil {
		fmt.Println("Could not start worker error:", err.Error())
		return
	}

	app.Metrics.BuildInfo.Inc()
	app.RegisterWorkerRoutes(r)
	app.RegisterInfraRoutes(r)
	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	log.Info(fmt.Sprintf(`Started %s`, cfg.AppConfig.Name))

	if err := g.Wait(); err != nil {
		log.Error(err.Error())
	}

	log.Info(fmt.Sprintf(`Main done %s`, cfg.AppConfig.Name))
}

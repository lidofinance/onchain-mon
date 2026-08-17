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

func main() {
	// run returns the error so deferred cleanup still happens before os.Exit.
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

//nolint:funlen
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

	// Must be unique among instances
	if cfg.AppConfig.Source == "" {
		return errors.New("SOURCE must be set and unique among instances")
	}

	metricsStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	transport := &http.Transport{
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		MaxConnsPerHost:       12,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	notificationConfig, err := env.ReadNotificationConfig(cfg.AppConfig.Env, `notification.yaml`)
	if err != nil {
		return fmt.Errorf("load notification config: %w", err)
	}

	notificationChannels, err := env.NewNotificationChannels(
		log, notificationConfig, httpClient,
		metricsStore,
		cfg.AppConfig.BlockExplorer,
		cfg.AppConfig.Source,
		cfg.AppConfig.Env,
	)
	if err != nil {
		return fmt.Errorf("init notification channels: %w", err)
	}

	natsConsumerCount := 0
	for _, consumerCfg := range notificationConfig.Consumers {
		natsConsumerCount += len(consumerCfg.Subjects)
	}

	rds, err := redis.NewRedisClient(cfg.AppConfig.RedisConfig.URL, cfg.AppConfig.RedisConfig.DB, log, natsConsumerCount)
	if err != nil {
		return fmt.Errorf("create redis client: %w", err)
	}
	defer rds.Close()

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

	app := server.New(&cfg.AppConfig, log, metricsStore, js, natsClient)

	natsStreamName := `NatsStream`
	natStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       natsStreamName,
		Discard:    jetstream.DiscardOld,
		MaxAge:     10 * time.Minute,
		Subjects:   env.CollectNatsSubjects(notificationConfig),
		MaxMsgSize: nc.MaxMsgSize,
		Retention:  jetstream.InterestPolicy,
	})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		return fmt.Errorf("create %s stream: %w", natsStreamName, err)
	}
	log.Info(natsStreamName + " jetStream createdOrUpdated")

	consumers, err := consumer.NewConsumers(
		log,
		metricsStore,
		rds,
		cfg.AppConfig.Source,
		consumer.NewRepo(rds, cfg.AppConfig.QuorumSize),
		cfg.AppConfig.QuorumSize,
		notificationConfig,
		notificationChannels,
	)
	if err != nil {
		return fmt.Errorf("init consumers: %w", err)
	}

	worker := forwarder.New(
		cfg.AppConfig.Source,
		rds,
		consumers, natStream, log,
		&cfg.AppConfig.RedisConfig,
		notificationChannels,
	)

	if err = worker.ConsumeFindings(gCtx, g); err != nil {
		return fmt.Errorf("start findings consumer: %w", err)
	}

	app.Metrics.BuildInfo.Inc()
	app.RegisterWorkerRoutes(r)
	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)

	log.Info("Started forwarder")

	if err := g.Wait(); err != nil {
		return fmt.Errorf("%s stopped: %w", cfg.AppConfig.Name, err)
	}

	log.Info("Main done forwarder")

	return nil
}

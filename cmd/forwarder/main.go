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
	"golang.org/x/time/rate"

	"github.com/lidofinance/onchain-mon/internal/app/forwarder"
	"github.com/lidofinance/onchain-mon/internal/app/server"
	"github.com/lidofinance/onchain-mon/internal/connectors/logger"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	nc "github.com/lidofinance/onchain-mon/internal/connectors/nats"
	"github.com/lidofinance/onchain-mon/internal/connectors/redis"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
)

const maxMsgSize = 4 * 1024 * 1024 // 4 Mb
const ConsumerGroupExists = "BUSYGROUP Consumer Group name already exists"

//nolint:funlen
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	defer stop()
	g, gCtx := errgroup.WithContext(ctx)

	cfg, envErr := env.Read("")
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log, sentryClient, logErr := logger.New(&cfg.AppConfig)
	if logErr != nil {
		fmt.Println("New log error:", logErr.Error())
		return
	}
	if sentryClient != nil {
		defer sentryClient.Flush(2 * time.Second)
	}

	// Must be unique among instances
	if cfg.AppConfig.Source == "" {
		log.Error("Env Source must be set and unique among instances")
		return
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
		log.Error(fmt.Sprintf("Error loading consumer's config: %v", err))
		return
	}

	notificationChannels, err := env.NewNotificationChannels(
		log, notificationConfig, httpClient,
		metricsStore,
		cfg.AppConfig.BlockExplorer,
		&cfg.AppConfig.RedisConfig,
	)
	if err != nil {
		log.Error(fmt.Sprintf("Could not init notification channels: %v", err))
		return
	}

	natsConsumerCount := 0
	for _, consumerCfg := range notificationConfig.Consumers {
		natsConsumerCount += len(consumerCfg.Subjects)
	}

	rds, err := redis.NewRedisClient(cfg.AppConfig.RedisConfig.URL, cfg.AppConfig.RedisConfig.DB, log, natsConsumerCount)
	if err != nil {
		log.Error(fmt.Sprintf(`create redis client error: %v`, err))
		return
	}
	defer rds.Close()

	redisStreamClient, err := redis.NewStreamClient(
		cfg.AppConfig.RedisConfig.URL,
		cfg.AppConfig.RedisConfig.DB,
		notificationChannels.Count(),
		log,
	)
	if err != nil {
		log.Error(fmt.Sprintf(`create redis stream client error: %v`, err))
		return
	}
	defer redisStreamClient.Close()

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

	app := server.New(&cfg.AppConfig, notificationConfig, log, metricsStore, js, natsClient)

	natsStreamName := `NatsStream`
	natStream, err := js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:       natsStreamName,
		Discard:    jetstream.DiscardOld,
		MaxAge:     10 * time.Minute,
		Subjects:   env.CollectNatsSubjects(notificationConfig),
		MaxMsgSize: maxMsgSize,
		Retention:  jetstream.InterestPolicy,
	})
	if err != nil && !errors.Is(err, nats.ErrStreamNameAlreadyInUse) {
		fmt.Printf("could not create %s stream error: %v\n", natsStreamName, err)
		return
	}
	log.Info(fmt.Sprintf("%s jetStream createdOrUpdated", natsStreamName))

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
		log.Error(fmt.Sprintf("Could not init consumers: %v", err))
		return
	}

	worker := forwarder.New(
		cfg.AppConfig.Source,
		rds,
		redisStreamClient,
		consumers, natStream, log,
		&cfg.AppConfig.RedisConfig,
		notificationChannels,
	)

	for _, sender := range notificationChannels.TelegramChannels {
		if err = rds.XGroupCreateMkStream(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), "$").Err(); err != nil {
			if err.Error() != ConsumerGroupExists {
				log.Error(fmt.Sprintf("Error creating %s stream: %v", sender.GetRedisStreamName(), err))
			}
		}

		tgLimiter := rate.NewLimiter(rate.Every(3*time.Second), 1)

		worker.SendFindings(gCtx, g,
			fmt.Sprintf("telegram-consumer-%s-%s", cfg.AppConfig.Source, sender.GetChannelID()),
			sender,
			tgLimiter,
		)
	}

	for _, sender := range notificationChannels.DiscordChannels {
		if err = rds.XGroupCreateMkStream(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), "$").Err(); err != nil {
			if err.Error() != ConsumerGroupExists {
				log.Error(fmt.Sprintf("Error creating %s stream: %v", sender.GetRedisStreamName(), err))
			}
		}

		discordLimiter := rate.NewLimiter(rate.Every(2*time.Second), 1)

		worker.SendFindings(gCtx, g,
			fmt.Sprintf("discord-consumer-%s-%s", cfg.AppConfig.Source, sender.GetChannelID()),
			sender,
			discordLimiter,
		)
	}

	for _, sender := range notificationChannels.OpsGenieChannels {
		if err = rds.XGroupCreateMkStream(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), "$").Err(); err != nil {
			if err.Error() != ConsumerGroupExists {
				log.Error(fmt.Sprintf("Error creating %s stream: %v", sender.GetRedisStreamName(), err))
			}
		}

		opsGenieLimiter := rate.NewLimiter(rate.Every(1*time.Second), 1)

		worker.SendFindings(gCtx, g,
			fmt.Sprintf("opsgenie-consumer-%s-%s", cfg.AppConfig.Source, sender.GetChannelID()),
			sender,
			opsGenieLimiter,
		)
	}

	if err = worker.ConsumeFindings(gCtx, g); err != nil {
		fmt.Println("Could not findings consumer:", err.Error())
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

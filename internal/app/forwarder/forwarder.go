package forwarder

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type worker struct {
	instance  string
	rdb       *redis.Client
	consumers []*consumer.Consumer

	stream jetstream.Stream
	log    *slog.Logger

	redisConfig          *env.RedisConfig
	notificationChannels *env.NotificationChannels
}

func New(
	instance string,
	rdb *redis.Client,
	consumers []*consumer.Consumer,
	stream jetstream.Stream,
	log *slog.Logger,
	redisConfig *env.RedisConfig,
	notificationChannels *env.NotificationChannels,
) *worker {
	w := &worker{
		instance:             instance,
		rdb:                  rdb,
		consumers:            consumers,
		stream:               stream,
		log:                  log,
		redisConfig:          redisConfig,
		notificationChannels: notificationChannels,
	}

	return w
}

func (w *worker) ConsumeFindings(ctx context.Context, g *errgroup.Group) error {
	connections := make([]jetstream.ConsumeContext, 0, len(w.consumers))
	for _, consumer := range w.consumers {
		con, err := w.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Durable:           consumer.GetName(),
			AckPolicy:         jetstream.AckExplicitPolicy,
			MaxAckPending:     1,
			FilterSubjects:    []string{consumer.GetTopic()},
			DeliverPolicy:     jetstream.DeliverNewPolicy,
			MaxDeliver:        10,
			InactiveThreshold: 2 * time.Hour,
		})

		if err != nil {
			return err
		}

		w.log.Info(fmt.Sprintf(`%s listens up %s`, consumer.GetName(), consumer.GetTopic()))

		conCtx, consumeErr := con.Consume(consumer.GetConsumeHandler(ctx))
		if consumeErr != nil {
			return consumeErr
		}

		connections = append(connections, conCtx)
	}

	g.Go(func() error {
		<-ctx.Done()
		for _, conCtx := range connections {
			conCtx.Stop()
		}
		return nil
	})

	return nil
}

func (w *worker) SendFindings(
	ctx context.Context,
	g *errgroup.Group,
	consumerName string,
	streamName string,
	groupName string,
	channelType registry.NotificationChannel,
	limiter *rate.Limiter,
) {
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				streams, err := w.rdb.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    groupName,
					Consumer: consumerName,
					Streams:  []string{streamName, ">"},
					Count:    1,
					Block:    1 * time.Second,
				}).Result()

				if err != nil {
					if errors.Is(err, redis.Nil) {
						continue
					}
					w.log.Error("Could not read stream", slog.String("stream", streamName), slog.String("err", err.Error()))
					continue
				}

				for _, str := range streams {
					for _, msg := range str.Messages {
						if limiter != nil && limiter.Wait(ctx) != nil {
							w.log.Info(fmt.Sprintf("Reached limit of message %s limit. sleeping", string(channelType)))
						}

						ct, ok := msg.Values["channelType"].(string)
						if !ok || ct != string(channelType) {
							w.log.Error("Invalid or mismatched channelType", slog.String("got", ct), slog.String("expected", string(channelType)))
							_ = w.rdb.XAck(ctx, streamName, groupName, msg.ID).Err()
							continue
						}

						channelID, ok := msg.Values["channelId"].(string)
						if !ok {
							w.log.Error("Missing channelId")
							_ = w.rdb.XAck(ctx, streamName, groupName, msg.ID).Err()
							continue
						}

						var sender notifiler.FindingSender
						switch channelType {
						case registry.Telegram:
							sender = w.notificationChannels.TelegramChannels[channelID]
						case registry.Discord:
							sender = w.notificationChannels.DiscordChannels[channelID]
						case registry.OpsGenie:
							sender = w.notificationChannels.OpsGenieChannels[channelID]
						}

						findingStr, ok := msg.Values["finding"].(string)
						if !ok {
							w.log.Error("Missing finding field")
							_ = w.rdb.XAck(ctx, streamName, groupName, msg.ID).Err()
							continue
						}

						var finding databus.FindingDtoJson
						if err := json.Unmarshal([]byte(findingStr), &finding); err != nil {
							w.log.Error("Failed to unmarshal finding", slog.String("err", err.Error()))
							_ = w.rdb.XAck(ctx, streamName, groupName, msg.ID).Err()
							continue
						}

						if err := sender.SendFinding(ctx, &finding); err != nil {
							var waitTime time.Duration
							switch channelType {
							case registry.Telegram:
								waitTime = 30 * time.Minute
							case registry.Discord:
								waitTime = 15 * time.Second
							case registry.OpsGenie:
								waitTime = 5 * time.Second
							}

							w.log.Warn(
								fmt.Sprintf("%s[%s] could not sent finding[%s]. Put onto queue again. Sleep(%.0f)", w.instance, channelType, finding.AlertId, waitTime.Seconds()),
								slog.String("err", err.Error()),
								slog.String("alertId", finding.AlertId),
								slog.String("name", finding.Name),
								slog.String("desc", finding.Description),
								slog.String("instance", w.instance),
								slog.String("bot-name", finding.BotName),
								slog.String("severity", string(finding.Severity)),
								slog.String("uniqueKey", finding.UniqueKey),
							)

							// Requeue for retry
							_ = w.rdb.XAdd(ctx, &redis.XAddArgs{
								Stream: streamName,
								Values: msg.Values,
							})

							// Went worker to sleep for a some time
							// No reason to flood TG, DG or OpsGenie.
							// Msg limit is reached
							time.Sleep(waitTime)
							continue
						}

						w.log.Info(
							fmt.Sprintf("%s[%s] sent finding[%s]", w.instance, channelType, finding.AlertId),
							slog.String("alertId", finding.AlertId),
							slog.String("name", finding.Name),
							slog.String("desc", finding.Description),
							slog.String("instance", w.instance),
							slog.String("bot-name", finding.BotName),
							slog.String("severity", string(finding.Severity)),
							slog.String("uniqueKey", finding.UniqueKey),
						)

						_ = w.rdb.XAck(ctx, streamName, groupName, msg.ID).Err()
					}
				}
			}
		}
	})
}

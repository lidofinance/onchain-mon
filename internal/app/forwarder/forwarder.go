package forwarder

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type worker struct {
	instance          string
	rdb               *redis.Client
	streamRedisClient *redis.Client
	consumers         []*consumer.Consumer

	stream jetstream.Stream
	log    *slog.Logger

	redisConfig          *env.RedisConfig
	notificationChannels *env.NotificationChannels
}

func New(
	instance string,
	rdb *redis.Client,
	streamRedisClient *redis.Client,
	consumers []*consumer.Consumer,
	stream jetstream.Stream,
	log *slog.Logger,
	redisConfig *env.RedisConfig,
	notificationChannels *env.NotificationChannels,
) *worker {
	w := &worker{
		instance:             instance,
		rdb:                  rdb,
		streamRedisClient:    streamRedisClient,
		consumers:            consumers,
		stream:               stream,
		log:                  log,
		redisConfig:          redisConfig,
		notificationChannels: notificationChannels,
	}

	return w
}

const StreamWorkerSleepInterval = 200 * time.Millisecond
const TelegramRelaxTime = 30 * time.Second
const RelaxTime = 5 * time.Second
const MaxAttempts = 10

func (w *worker) ConsumeFindings(ctx context.Context, g *errgroup.Group) error {
	connections := make([]jetstream.ConsumeContext, 0, len(w.consumers))
	for _, consumer := range w.consumers {
		maxAckPending := 1
		if consumer.ByQuorum() {
			maxAckPending = 6
		}

		con, err := w.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Durable:   consumer.GetName(),
			AckPolicy: jetstream.AckExplicitPolicy,
			// Telegram limit: ~20 msgs/min per bot
			// We run 3 instances that handle 6 messages in parallel = 18 sends max (safe 18 <= 20)
			// Extra messages are rate-limited and queued to Redis Streams for retry
			MaxAckPending:     maxAckPending,
			AckWait:           30 * time.Second,
			FilterSubjects:    []string{consumer.GetTopic()},
			DeliverPolicy:     jetstream.DeliverNewPolicy,
			MaxDeliver:        10,
			InactiveThreshold: 2 * time.Hour,
			BackOff: []time.Duration{
				1 * time.Second, 2 * time.Second,
				4 * time.Second, 8 * time.Second,
				16 * time.Second, 30 * time.Second,
			},
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
	sender notifiler.FindingSender,
	limiter *rate.Limiter,
) {
	g.Go(func() error {
		conn := w.streamRedisClient.Conn()
		defer conn.Close()

		for {
			select {
			case <-ctx.Done():
				return nil
			default:
				streams, err := conn.XReadGroup(ctx, &redis.XReadGroupArgs{
					Group:    sender.GetRedisConsumerGroupName(),
					Consumer: consumerName,
					Streams:  []string{sender.GetRedisStreamName(), ">"},
					Count:    10,
					Block:    5 * time.Second,
				}).Result()

				if err != nil {
					if errors.Is(err, redis.Nil) {
						continue
					}

					time.Sleep(StreamWorkerSleepInterval)
					continue
				}

				for _, str := range streams {
					for _, msg := range str.Messages {
						if limiter != nil && limiter.Wait(ctx) != nil {
							w.log.Info(fmt.Sprintf("Reached limit of message %s limit. sleeping", string(sender.GetType())))
						}

						quorumBy, ok := msg.Values["quorumBy"].(string)
						if !ok {
							w.log.Error("Missing quorumBy")
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						findingStr, ok := msg.Values["finding"].(string)
						if !ok {
							w.log.Error("Missing finding field")
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						var finding databus.FindingDtoJson
						if err := json.Unmarshal([]byte(findingStr), &finding); err != nil {
							w.log.Error(fmt.Sprintf("Failed to unmarshal finding %s", err.Error()))
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						key := fmt.Sprintf("%s_%s_%s_%s", sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), quorumBy, finding.UniqueKey)
						hash := sha256.Sum256([]byte(key))
						redisKey := fmt.Sprintf(`dropMsg:%s`, hex.EncodeToString(hash[:]))

						if sendErr := sender.SendFinding(ctx, &finding, quorumBy); sendErr != nil {
							count, incErr := conn.Incr(ctx, redisKey).Uint64()
							if incErr != nil {
								w.log.Error(fmt.Sprintf(`Could not increase countKey for %s: %v`, sender.GetRedisStreamName(), err),
									slog.String("alertId", finding.AlertId))
								_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
								_ = conn.Del(ctx, redisKey).Err()
								continue
							}

							if count >= MaxAttempts {
								w.log.Error(fmt.Sprintf("Archived maximum attempts for sending message %s", finding.AlertId))
								_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
								_ = conn.Del(ctx, redisKey).Err()
								continue
							}

							// Requeue for retry
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							if errors.Is(sendErr, notifiler.ErrRateLimited) {
								// specially block goroutine, No sense for sending findings when we reached limits
								if sender.GetType() == registry.Telegram {
									time.Sleep(TelegramRelaxTime)
								} else {
									time.Sleep(RelaxTime)
								}

								_ = conn.XAdd(ctx, &redis.XAddArgs{
									Stream: sender.GetRedisStreamName(),
									Values: msg.Values,
								})
							}

							w.log.Warn(
								fmt.Sprintf("%s[%s] could not sent finding[%s]. Put onto queue again", w.instance, sender.GetType(), finding.AlertId),
								slog.String("err", sendErr.Error()),
								slog.String("alertId", finding.AlertId),
								slog.String("name", finding.Name),
								slog.String("desc", finding.Description),
								slog.String("instance", w.instance),
								slog.String("bot-name", finding.BotName),
								slog.String("severity", string(finding.Severity)),
								slog.String("uniqueKey", finding.UniqueKey),
							)

							continue
						}

						w.log.Info(
							fmt.Sprintf("%s[%s] sent finding[%s]", w.instance, sender.GetType(), finding.AlertId),
							slog.String("alertId", finding.AlertId),
							slog.String("name", finding.Name),
							slog.String("desc", finding.Description),
							slog.String("instance", w.instance),
							slog.String("bot-name", finding.BotName),
							slog.String("severity", string(finding.Severity)),
							slog.String("uniqueKey", finding.UniqueKey),
						)

						_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
						_ = conn.Del(ctx, redisKey).Err()
					}
				}
			}
		}
	})
}

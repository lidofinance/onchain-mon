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

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
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
const MaxAttemptsTLL = 10 * time.Minute
const IdempotentKeyTTL = 30 * time.Minute

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
) {
	w.log.Info(fmt.Sprintf("%s in %s by %s is started", consumerName, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName()))

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
					Block:    1 * time.Second,
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
						byQuorum, ok := msg.Values["byQuorum"].(string)
						if !ok {
							w.log.Error("Missing `byQuorum` field")
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						source, ok := msg.Values["source"].(string)
						if !ok {
							w.log.Error("Missing `source` field")
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						findingStr, ok := msg.Values["finding"].(string)
						if !ok {
							w.log.Error("Missing `finding` field")
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						var finding databus.FindingDtoJson
						if err := json.Unmarshal([]byte(findingStr), &finding); err != nil {
							w.log.Error(fmt.Sprintf("Failed to unmarshal finding %s", err.Error()))
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						key := fmt.Sprintf("%s_%s_%s", sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), finding.UniqueKey)
						// When a finding is built by quorum, we must avoid adding extra keys,
						// since it should remain unique across workers on different replicas.
						if byQuorum == "0" {
							key += "_" + source
						}

						hash := sha256.Sum256([]byte(key))
						attemptsKey := fmt.Sprintf(`dropMsg:%s`, hex.EncodeToString(hash[:]))
						idempotentKey := `published:` + hex.EncodeToString(hash[:])

						ok, err := conn.SetNX(ctx, idempotentKey, 1, IdempotentKeyTTL).Result()
						if err != nil {
							w.log.Error("redis SetNX failed", slog.String("err", err.Error()))
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}
						if !ok {
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
							continue
						}

						if sendErr := sender.SendFinding(ctx, &finding, source); sendErr != nil {
							w.log.Error(
								fmt.Sprintf("%s[%s] could not sent finding[%s]", w.instance, sender.GetType(), finding.AlertId),
								slog.String("err", sendErr.Error()),
								slog.String("alertId", finding.AlertId),
								slog.String("name", finding.Name),
								slog.String("desc", finding.Description),
								slog.String("instance", w.instance),
								slog.String("bot-name", finding.BotName),
								slog.String("severity", string(finding.Severity)),
								slog.String("uniqueKey", finding.UniqueKey),
							)

							count, incErr := conn.Incr(ctx, attemptsKey).Uint64()
							w.log.Info(fmt.Sprintf("Count %d ", count))

							if incErr == nil && count == 1 {
								conn.Expire(ctx, attemptsKey, MaxAttemptsTLL)
							}

							if incErr != nil {
								w.log.Error(fmt.Sprintf(`Could not increase countKey for %s: %v`, sender.GetRedisStreamName(), err),
									slog.String("alertId", finding.AlertId))
								_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
								_ = conn.Del(ctx, attemptsKey).Err()
								_ = conn.Del(ctx, idempotentKey).Err()
								continue
							}

							if count >= MaxAttempts {
								w.log.Error(fmt.Sprintf("Achieved maximum attempts for sending message %s", finding.AlertId))
								_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()
								_ = conn.Del(ctx, attemptsKey).Err()
								_ = conn.Del(ctx, idempotentKey).Err()
								continue
							}

							_ = conn.Del(ctx, idempotentKey).Err()
							// Requeue for retry
							_ = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err()

							var rle *notifiler.RateLimitedError
							if errors.As(sendErr, &rle) {
								backoff := rle.ResetAfter + 500*time.Millisecond
								time.Sleep(backoff)

								if err = conn.XAdd(ctx, &redis.XAddArgs{
									Stream: sender.GetRedisStreamName(),
									Values: msg.Values,
								}).Err(); err != nil {
									w.log.Error(fmt.Sprintf("Failed to reput message to redis stream %s: %v", sender.GetRedisStreamName(), err))
								}

								w.log.Info(
									fmt.Sprintf("%s[%s] put finding[%s] onto queue again", w.instance, sender.GetType(), finding.AlertId),
									slog.String("err", sendErr.Error()),
									slog.String("alertId", finding.AlertId),
									slog.String("name", finding.Name),
									slog.String("desc", finding.Description),
									slog.String("instance", w.instance),
									slog.String("bot-name", finding.BotName),
									slog.String("severity", string(finding.Severity)),
									slog.String("uniqueKey", finding.UniqueKey),
								)
							}

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

						if err = conn.XAck(ctx, sender.GetRedisStreamName(), sender.GetRedisConsumerGroupName(), msg.ID).Err(); err != nil {
							w.log.Error("redis XAck failed", slog.String("err", err.Error()))
						}
						_ = conn.Del(ctx, attemptsKey).Err()
					}
				}
			}
		}
	})
}

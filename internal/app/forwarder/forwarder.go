package forwarder

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type carrier struct {
	Name      string
	notifiler notifiler.FindingSender
	channel   string
}

type findingCarrier struct {
	carrier
	findingSeveritySet registry.FindingMapping
}

type worker struct {
	filterSubject string

	stream  jetstream.Stream
	log     *slog.Logger
	metrics *metrics.Store
}

type findingWorker struct {
	quorum      uint64
	redisClient *redis.Client
	cache       *expirable.LRU[string, uint]
	worker
	carriers []findingCarrier
}

type FindingWorkerOptions func(worker *findingWorker)

var statusTemplate = "%s:finding:%s:status"
var countTemplate = "%s:finding:%s:count"

type Status string

const (
	StatusNotSend Status = "not_send"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
)

const (
	TTLMins10 = 10 * time.Minute
)

func WithFindingConsumer(
	notifier notifiler.FindingSender,
	consumerName string,
	findingSeveritySet registry.FindingMapping,
	channel string,
) FindingWorkerOptions {
	return func(w *findingWorker) {
		w.carriers = append(w.carriers, findingCarrier{
			carrier: carrier{
				Name:      consumerName,
				notifiler: notifier,
				channel:   channel,
			},
			findingSeveritySet: findingSeveritySet,
		})
	}
}

func NewFindingWorker(
	log *slog.Logger,
	metricsStore *metrics.Store,
	cache *expirable.LRU[string, uint],

	redisClient *redis.Client,
	stream jetstream.Stream,

	filterSubject string,
	quorum uint,

	options ...FindingWorkerOptions,
) *findingWorker {
	w := &findingWorker{
		redisClient: redisClient,
		quorum:      uint64(quorum),
		cache:       cache,
		worker: worker{
			filterSubject: filterSubject,
			stream:        stream,
			log:           log,
			metrics:       metricsStore,
		},
	}

	for _, option := range options {
		option(w)
	}

	return w
}

func (w *findingWorker) Run(ctx context.Context, g *errgroup.Group) error {
	type Consumer struct {
		name    string
		handler func(msg jetstream.Msg)
	}

	consumers := make([]Consumer, 0, len(w.carriers))
	for _, consumer := range w.carriers {
		consumers = append(consumers, Consumer{
			name: consumer.Name,
			handler: func(msg jetstream.Msg) {
				finding := new(databus.FindingDtoJson)

				if alertErr := json.Unmarshal(msg.Data(), finding); alertErr != nil {
					w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
					w.terminateMessage(msg)
					return
				}
				defer func() {
					finding = nil
				}()

				if _, ok := consumer.findingSeveritySet[finding.Severity]; !ok {
					w.ackMessage(msg)
					return
				}

				if finding.Severity == databus.SeverityUnknown {
					if sendErr := consumer.notifiler.SendFinding(ctx, finding); sendErr != nil {
						w.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr),
							slog.Attr{
								Key:   "alertID",
								Value: slog.StringValue(finding.AlertId),
							})
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
						w.nackMessage(msg)
						return
					}

					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
					w.ackMessage(msg)
					return
				}

				key := findingToUniqueHash(finding)

				countKey := fmt.Sprintf(countTemplate, consumer.Name, key)
				statusKey := fmt.Sprintf(statusTemplate, consumer.Name, key)

				var (
					count uint64
					err   error
				)

				if !w.cache.Contains(countKey) {
					w.cache.Add(countKey, uint(1))

					count, err = w.redisClient.Incr(ctx, countKey).Uint64()
					if err != nil {
						w.log.Error(fmt.Sprintf(`Could not increase key value: %v`, err))
						w.metrics.RedisErrors.Inc()
						w.nackMessage(msg)
						return
					}

					if count == 1 {
						if err := w.redisClient.Expire(ctx, countKey, TTLMins10).Err(); err != nil {
							w.log.Error(fmt.Sprintf(`Could not set expire time: %v`, err))
							w.metrics.RedisErrors.Inc()

							if _, err := w.redisClient.Decr(ctx, countKey).Result(); err != nil {
								w.metrics.RedisErrors.Inc()
								w.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
							}

							w.nackMessage(msg)
							return
						}
					}
				} else {
					v, _ := w.cache.Get(countKey)
					w.cache.Add(countKey, v+1)

					count, err = w.redisClient.Get(ctx, countKey).Uint64()
					if err != nil {
						w.log.Error(fmt.Sprintf(`Could not get key value: %v`, err))
						w.metrics.RedisErrors.Inc()
						w.nackMessage(msg)
						return
					}
				}

				touchTimes, _ := w.cache.Get(countKey)

				msgInfo := fmt.Sprintf("%s: AlertId %s read %d times. %s...%s", consumer.Name, finding.AlertId, touchTimes, key[0:4], key[len(key)-4:])
				if finding.BlockNumber != nil {
					msgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
				}

				// TODO add finding.blockNumber
				w.log.Info(msgInfo,
					slog.Attr{
						Key:   `desc`,
						Value: slog.StringValue(finding.Description),
					},
					slog.Attr{
						Key:   `name`,
						Value: slog.StringValue(finding.Name),
					},
					slog.Attr{
						Key:   `alertId`,
						Value: slog.StringValue(finding.AlertId),
					},
					slog.Attr{
						Key:   `severity`,
						Value: slog.StringValue(string(finding.Severity)),
					},
					slog.Attr{
						Key:   `hash`,
						Value: slog.StringValue(key),
					},
				)

				/*if count == 1 && touchTimes == 10 {
					finding.Severity = proto.Finding_UNKNOWN
					finding.Description += fmt.Sprintf("\n\nWarning: Could not collect quorum. Finding.Severity downgraded to UNKNOWN")

					if sendErr := consumer.notifiler.SendFinding(ctx, finding); sendErr != nil {
						w.log.Error(fmt.Sprintf(`Could not send bot-finding when did not collect quorum: %v`, sendErr), slog.Attr{
							Key:   "alertID",
							Value: slog.StringValue(finding.AlertId),
						})
					}

					w.ackMessage(msg)
					return
				}*/

				if count >= w.quorum {
					status, err := w.GetStatus(ctx, statusKey)
					if err != nil {
						w.log.Error(fmt.Sprintf(`Could not get notification status: %v`, err))
						w.metrics.RedisErrors.Inc()
						w.nackMessage(msg)
						return
					}

					if status == StatusSending {
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()

						w.log.Info(fmt.Sprintf("Another instance is sending finding: %s", finding.AlertId))
						return
					}

					if status == StatusSent {
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
						w.ackMessage(msg)

						w.cache.Remove(countKey)
						w.log.Info(fmt.Sprintf("Another instance already sent finding: %s", finding.AlertId))
						return
					}

					if status == StatusNotSend {
						readyToSend, err := w.SetSendingStatus(ctx, countKey, statusKey)
						if err != nil {
							w.log.Error(fmt.Sprintf(`Could not check notification status for AlertID: %s: %v`, finding.AlertId, err))
							w.metrics.RedisErrors.Inc()
							w.nackMessage(msg)
							return
						}

						if readyToSend {
							if sendErr := consumer.notifiler.SendFinding(ctx, finding); sendErr != nil {
								w.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr), slog.Attr{
									Key:   "alertID",
									Value: slog.StringValue(finding.AlertId),
								})

								count, err := w.redisClient.Decr(ctx, countKey).Result()
								if err != nil {
									w.metrics.RedisErrors.Inc()
									w.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
								} else if count <= 0 {
									if err = w.redisClient.Del(ctx, countKey).Err(); err != nil {
										w.metrics.RedisErrors.Inc()
										w.log.Error(fmt.Sprintf(`Could not delete countKey %s: %v`, countKey, err))
									}
								}

								if err = w.redisClient.Del(ctx, statusKey).Err(); err != nil {
									w.metrics.RedisErrors.Inc()
									w.log.Error(fmt.Sprintf(`Could not delete statusKey %s: %v`, statusKey, err))
								}

								w.cache.Remove(countKey)

								w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
								w.nackMessage(msg)
								return
							}

							w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
							w.ackMessage(msg)

							w.log.Info(fmt.Sprintf("Consumer: %s successfully sent finding to %s, alertId %s", consumer.Name, consumer.channel, finding.AlertId),
								slog.Attr{
									Key:   `alertId`,
									Value: slog.StringValue(finding.AlertId),
								},
								slog.Attr{
									Key:   `name`,
									Value: slog.StringValue(finding.Name),
								},
								slog.Attr{
									Key:   `desc`,
									Value: slog.StringValue(finding.Description),
								},
							)

							if err := w.SeNotificationStatus(ctx, key, StatusSent); err != nil {
								w.metrics.RedisErrors.Inc()
								w.log.Error(fmt.Sprintf(`Could not set notification StatusSent: %s`, err.Error()))
							}
						}
					}
				}
			},
		})
	}

	connections := make([]jetstream.ConsumeContext, 0, len(consumers))
	for _, consumer := range consumers {
		con, err := w.stream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Durable:        consumer.name,
			AckPolicy:      jetstream.AckExplicitPolicy,
			MaxAckPending:  1,
			FilterSubjects: []string{w.filterSubject},
			DeliverPolicy:  jetstream.DeliverNewPolicy,
			MaxDeliver:     10,
		})

		if err != nil {
			return err
		}

		w.log.Info(fmt.Sprintf(`Consumer %s created or updated successfully`, consumer.name))

		conCtx, consumeErr := con.Consume(consumer.handler)
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

func (w *worker) terminateMessage(msg jetstream.Msg) {
	if termErr := msg.Term(); termErr != nil {
		w.log.Error(fmt.Sprintf(`Could not term msg: %v`, termErr))
	}
}

func (w *worker) nackMessage(msg jetstream.Msg) {
	if nackErr := msg.Nak(); nackErr != nil {
		w.log.Error(fmt.Sprintf(`Could not nack msg: %v`, nackErr))
	}
}

func (w *worker) ackMessage(msg jetstream.Msg) {
	if ackErr := msg.Ack(); ackErr != nil {
		w.log.Error(fmt.Sprintf(`Could not ack msg: %v`, ackErr))
	}
}

func computeSHA256Hash(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func findingToUniqueHash(f *databus.FindingDtoJson) string {
	var buffer bytes.Buffer

	if f.UniqueKey != nil {
		buffer.WriteString(f.Team)
		buffer.WriteString(f.BotName)
		buffer.WriteString(*f.UniqueKey)
	} else {
		buffer.WriteString(f.Team)
		buffer.WriteString(f.BotName)
		buffer.WriteString(f.AlertId)
		buffer.WriteString(f.Name)
		buffer.WriteString(string(f.Severity))
	}

	return computeSHA256Hash(buffer.Bytes())
}

func (w *findingWorker) SetSendingStatus(ctx context.Context, countKey, statusKey string) (bool, error) {
	luaScript := `
        local count = tonumber(redis.call("GET", KEYS[1]))
        if count and count >= tonumber(ARGV[1]) then
            local status = redis.call("GET", KEYS[2])
            if not status or status == ARGV[2] then
                redis.call("SET", KEYS[2], ARGV[3])
                return 1
            end
        end
        return 0
    `

	res, err := w.redisClient.Eval(ctx, luaScript, []string{countKey, statusKey}, w.quorum, string(StatusNotSend), string(StatusSent)).Result()
	if err != nil {
		return false, fmt.Errorf(`could not get notification_sent_status: %v`, err)
	}

	sending, ok := res.(int64)
	if !ok {
		return false, fmt.Errorf("unexpected notification_sent_status: %v", res)
	}

	return sending == 1, nil
}

func (w *findingWorker) SeNotificationStatus(ctx context.Context, statusKey string, status Status) error {
	return w.redisClient.Set(ctx, statusKey, string(status), TTLMins10).Err()
}

func (w *findingWorker) GetStatus(ctx context.Context, statusKey string) (Status, error) {
	status, err := w.redisClient.Get(ctx, statusKey).Result()
	if errors.Is(err, redis.Nil) {
		return StatusNotSend, nil
	} else if err != nil {
		return "", fmt.Errorf("could not get status for key %s: %w", statusKey, err)
	}

	return Status(status), nil
}

package worker

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/go-redis/redis/v8"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/generated/proto"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/pkg/notifiler"
	"github.com/lidofinance/finding-forwarder/internal/utils/registry"
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

type alertCarrier struct {
	carrier
	alertSeveritySet registry.AlertMapping
}

type worker struct {
	filterSubject string

	stream  jetstream.Stream
	log     *slog.Logger
	metrics *metrics.Store
}

type findingWorker struct {
	quorum uint
	redis  *redis.Client
	worker
	carriers []findingCarrier
}

type alertWorker struct {
	worker
	carriers []alertCarrier
}

type FindingWorkerOptions func(worker *findingWorker)
type AlertWorkerOptions func(worker *alertWorker)

var statusTemplate = "%s:finding:%s:status"
var countTemplate = "%s:finding:%s:count"

type Status string

const (
	StatusNotSend Status = "not_send"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
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

func WithAlertConsumer(
	notifier notifiler.FindingSender,
	consumerName string,
	alertSeveritySet registry.AlertMapping,
	channel string,
) AlertWorkerOptions {
	return func(w *alertWorker) {
		w.carriers = append(w.carriers, alertCarrier{
			carrier: carrier{
				Name:      consumerName,
				notifiler: notifier,
				channel:   channel,
			},
			alertSeveritySet: alertSeveritySet,
		})
	}
}

func NewFindingWorker(
	filterSubject string,
	quorum uint,
	redis *redis.Client,
	stream jetstream.Stream,
	log *slog.Logger,
	metricsStore *metrics.Store,

	options ...FindingWorkerOptions,
) *findingWorker {
	w := &findingWorker{
		redis:  redis,
		quorum: quorum,
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

func NewAlertWorker(
	filterSubject string,

	stream jetstream.Stream,
	log *slog.Logger,
	metricsStore *metrics.Store,

	options ...AlertWorkerOptions,
) *alertWorker {
	w := &alertWorker{
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
				finding := new(proto.Finding)

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

				if finding.Severity == proto.Finding_UNKNOWN {
					if sendErr := consumer.notifiler.SendFinding(ctx, finding); sendErr != nil {
						w.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr),
							slog.With(slog.Attr{
								Key:   "alertID",
								Value: slog.StringValue(finding.AlertId),
							}))
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
						w.nackMessage(msg)
						return
					}

					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
					w.ackMessage(msg)
					return
				}

				// TODO to think on better hashing fields
				key := computeSHA256Hash(msg.Data())

				countKey := fmt.Sprintf(countTemplate, consumer.Name, key)
				statusKey := fmt.Sprintf(statusTemplate, consumer.Name, key)

				count, err := w.redis.Incr(ctx, countKey).Result()
				if err != nil {
					w.log.Error(fmt.Sprintf(`Could not increase key value: %v`, err))
					w.metrics.RedisErrors.Inc()
					w.nackMessage(msg)
					return
				}

				// TODO add finding.blockNumber
				w.log.InfoContext(ctx, fmt.Sprintf("Consumer: %s AlertId %s read %d times", consumer.Name, finding.AlertId, count))

				if count == 1 {
					if err := w.redis.Expire(ctx, countKey, 5*time.Minute).Err(); err != nil {
						w.log.Error(fmt.Sprintf(`Could not set expire time: %v`, err))
						w.metrics.RedisErrors.Inc()

						if _, err := w.redis.Decr(ctx, countKey).Result(); err != nil {
							w.metrics.RedisErrors.Inc()
							w.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
						}

						w.nackMessage(msg)
						return
					}
				}

				if count >= int64(w.quorum) {
					status, err := w.GetStatus(ctx, statusKey)
					if err != nil {
						w.log.Error(fmt.Sprintf(`Could not get notification status: %v`, err))
						w.metrics.RedisErrors.Inc()
						w.nackMessage(msg)
						return
					}

					if status == StatusSending {
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
						w.ackMessage(msg)

						w.log.Info(fmt.Sprintf("Another instance is sending finding: %s", finding.AlertId))
						return
					}

					if status == StatusSent {
						w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
						w.ackMessage(msg)

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
								w.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr), slog.With(slog.Attr{
									Key:   "alertID",
									Value: slog.StringValue(finding.AlertId),
								}))

								count, err := w.redis.Decr(ctx, countKey).Result()
								if err != nil {
									w.metrics.RedisErrors.Inc()
									w.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
								} else if count <= 0 {
									if err = w.redis.Del(ctx, countKey).Err(); err != nil {
										w.metrics.RedisErrors.Inc()
										w.log.Error(fmt.Sprintf(`Could not delete countKey %s: %v`, countKey, err))
									}
								}

								if err = w.redis.Del(ctx, statusKey).Err(); err != nil {
									w.metrics.RedisErrors.Inc()
									w.log.Error(fmt.Sprintf(`Could not delete statusKey %s: %v`, statusKey, err))
								}

								w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
								w.nackMessage(msg)
								return
							}

							w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
							w.ackMessage(msg)

							w.log.Info(fmt.Sprintf("Consumer: %s succefully sent finding to %s, alertId %s", consumer.Name, consumer.channel, finding.AlertId),
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
								w.log.Error(fmt.Sprintf(`Could not set notificaton StatusSent: %v`, err))
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

func (w *alertWorker) Run(ctx context.Context, g *errgroup.Group) error {
	type Consumer struct {
		name    string
		handler func(msg jetstream.Msg)
	}

	consumers := make([]Consumer, 0, len(w.carriers))
	for _, consumer := range w.carriers {
		consumers = append(consumers, Consumer{
			name: consumer.Name,
			handler: func(msg jetstream.Msg) {
				alert := new(models.Alert)

				if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
					w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
					w.terminateMessage(msg)
					return
				}
				defer func() {
					alert = nil
				}()

				if _, ok := consumer.alertSeveritySet[alert.Severity]; !ok {
					w.ackMessage(msg)
					return
				}

				if sendErr := consumer.notifiler.SendAlert(ctx, alert); sendErr != nil {
					w.log.Error(fmt.Sprintf(`Could not send forta-finding: %v`, sendErr), slog.With(slog.Attr{
						Key:   "alertID",
						Value: slog.StringValue(alert.AlertID),
					}))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusFail}).Inc()
					w.nackMessage(msg)
					return
				}

				w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: consumer.channel, metrics.Status: metrics.StatusOk}).Inc()
				w.ackMessage(msg)
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

	res, err := w.redis.Eval(ctx, luaScript, []string{countKey, statusKey}, w.quorum, string(StatusNotSend), string(StatusSent)).Result()
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
	return w.redis.Set(ctx, statusKey, string(status), 5*time.Minute).Err()
}

func (w *findingWorker) GetStatus(ctx context.Context, statusKey string) (Status, error) {
	status, err := w.redis.Get(ctx, statusKey).Result()
	if errors.Is(err, redis.Nil) {
		return StatusNotSend, nil
	} else if err != nil {
		return "", fmt.Errorf("could not get status for key %s: %w", statusKey, err)
	}

	return Status(status), nil
}

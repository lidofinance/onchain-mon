package worker

import (
	"context"
	"fmt"
	"log/slog"

	"golang.org/x/sync/errgroup"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/pkg/notifiler"
)

type telegramCarrier struct {
	Name      string
	notifiler notifiler.Telegram
}

type opsGeniaCarrier struct {
	Name      string
	notifiler notifiler.OpsGenia
}

type discordCarrier struct {
	Name      string
	notifiler notifiler.Discord
}

type worker struct {
	filterSubject string

	stream  jetstream.Stream
	log     *slog.Logger
	metrics *metrics.Store

	telegramConsumers []telegramCarrier
	opsGeniaConsumers []opsGeniaCarrier
	discordConsumers  []discordCarrier
}

const (
	Telegram = `Telegram`
	Discord  = `Discord`
	OpsGenie = `OpsGenie`
)

// WorkerOptions defines a function type for configuring ServerConfigBuilder.
type WorkerOptions func(worker *worker)

func WithTelegram(telegram notifiler.Telegram, consumerName string) WorkerOptions {
	return func(w *worker) {
		w.telegramConsumers = append(w.telegramConsumers, telegramCarrier{
			Name:      consumerName,
			notifiler: telegram,
		})
	}
}

func WithDiscord(discord notifiler.Discord, consumerName string) WorkerOptions {
	return func(w *worker) {
		w.discordConsumers = append(w.discordConsumers, discordCarrier{
			Name:      consumerName,
			notifiler: discord,
		})
	}
}

func WithOpsGenia(opsGenia notifiler.OpsGenia, consumerName string) WorkerOptions {
	return func(w *worker) {
		w.opsGeniaConsumers = append(w.opsGeniaConsumers, opsGeniaCarrier{
			Name:      consumerName,
			notifiler: opsGenia,
		})
	}
}

func NewWorker(
	filterSubject string,

	stream jetstream.Stream,
	log *slog.Logger,
	metricsStore *metrics.Store,

	options ...WorkerOptions,
) *worker {
	w := &worker{
		filterSubject: filterSubject,
		stream:        stream,
		log:           log,
		metrics:       metricsStore,
	}

	for _, option := range options {
		option(w)
	}

	return w
}

func (w *worker) Run(ctx context.Context, g *errgroup.Group) error {
	type Consumer struct {
		name    string
		handler func(msg jetstream.Msg)
	}

	consumers := make([]Consumer, 0, len(w.telegramConsumers)+len(w.discordConsumers)+len(w.opsGeniaConsumers))
	for _, telegramConsumer := range w.telegramConsumers {
		consumers = append(consumers, Consumer{
			name: telegramConsumer.Name,
			handler: func(msg jetstream.Msg) {
				alert := new(models.Alert)

				if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
					w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Telegram, metrics.Status: metrics.StatusFail}).Inc()
					w.terminateMessage(msg)
					return
				}
				defer func() {
					alert = nil
				}()

				if sendErr := telegramConsumer.notifiler.SendMessage(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
					w.log.Error(fmt.Sprintf(`Could not send finding: %v`, sendErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Telegram, metrics.Status: metrics.StatusFail}).Inc()
					w.nackMessage(msg)
					return
				}

				w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Telegram, metrics.Status: metrics.StatusOk}).Inc()
				w.ackMessage(msg)
			},
		})
	}

	for _, opsGeniaConsumer := range w.opsGeniaConsumers {
		consumers = append(consumers, Consumer{
			name: opsGeniaConsumer.Name,
			handler: func(msg jetstream.Msg) {
				var alert models.Alert
				if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGenie, metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
					w.terminateMessage(msg)
					return
				}

				opsGeniaPriority := ""
				switch alert.Severity {
				case models.AlertSeverityCRITICAL:
					opsGeniaPriority = "P2"
				case models.AlertSeverityHIGH:
					opsGeniaPriority = "P3"
				}

				if opsGeniaPriority == "" {
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGenie, metrics.Status: metrics.StatusOk}).Inc()
					w.ackMessage(msg)
					return
				}

				if sendErr := opsGeniaConsumer.notifiler.SendMessage(ctx, alert.Name, alert.Description,
					alert.AlertID, opsGeniaPriority); sendErr != nil {
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGenie, metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf(`Could not send finding to OpsGenia: %s`, sendErr.Error()))
					w.nackMessage(msg)
					return
				}

				w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGenie, metrics.Status: metrics.StatusOk}).Inc()
				w.ackMessage(msg)
			},
		})
	}

	for _, discordConsumer := range w.discordConsumers {
		consumers = append(consumers, Consumer{
			name: discordConsumer.Name,
			handler: func(msg jetstream.Msg) {
				alert := new(models.Alert)

				if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
					w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Discord, metrics.Status: metrics.StatusFail}).Inc()
					w.terminateMessage(msg)
					return
				}
				defer func() {
					alert = nil
				}()

				if sendErr := discordConsumer.notifiler.SendMessage(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
					w.log.Error(fmt.Sprintf(`Could not send finding: %v`, sendErr))
					w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Discord, metrics.Status: metrics.StatusFail}).Inc()
					w.nackMessage(msg)
					return
				}

				w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: Discord, metrics.Status: metrics.StatusOk}).Inc()
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
		})
		if err != nil {
			return err
		}

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

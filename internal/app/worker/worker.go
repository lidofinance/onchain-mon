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
	"github.com/lidofinance/finding-forwarder/internal/utils/registry"
)

type carrier struct {
	Name        string
	SeveritySet registry.AlertMapping
	notifiler   notifiler.FindingSender
	channel     string
}

type worker struct {
	filterSubject string

	stream  jetstream.Stream
	log     *slog.Logger
	metrics *metrics.Store

	carriers []carrier
}

// WorkerOptions defines a function type for configuring ServerConfigBuilder.
type WorkerOptions func(worker *worker)

func WithConsumer(notifier notifiler.FindingSender, consumerName string, severitySet registry.AlertMapping, channel string) WorkerOptions {
	return func(w *worker) {
		w.carriers = append(w.carriers, carrier{
			Name:        consumerName,
			SeveritySet: severitySet,
			notifiler:   notifier,
			channel:     channel,
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

				if _, ok := consumer.SeveritySet[alert.Severity]; !ok {
					w.ackMessage(msg)
					return
				}

				if sendErr := consumer.notifiler.SendFinding(ctx, alert); sendErr != nil {
					w.log.Error(fmt.Sprintf(`Could not send finding: %v`, sendErr))
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

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

type worker struct {
	log       *slog.Logger
	metrics   *metrics.Store
	jetStream jetstream.Stream
	telegram  notifiler.Telegram
	opsGenia  notifiler.OpsGenia
	discord   notifiler.Discord
}

const (
	Telegram  = `Telegram`
	Discrod   = `Discrod`
	OpsGeniea = `OpsGeniea`
)

func NewWorker(log *slog.Logger, metricsStore *metrics.Store, jetStream jetstream.Stream,
	telegram notifiler.Telegram, opsGenia notifiler.OpsGenia,
	discord notifiler.Discord) *worker {
	return &worker{
		log:       log,
		metrics:   metricsStore,
		jetStream: jetStream,
		telegram:  telegram,
		opsGenia:  opsGenia,
		discord:   discord,
	}
}

// TODO right now it's common alert dispatcher
// In neaerst future we have to create to each team their own worker line OZ defender
func (w *worker) Run(ctx context.Context, g *errgroup.Group) error {
	consumers := []struct {
		name    string
		handler func(msg jetstream.Msg)
	}{
		{
			name: "Dicorder",
			handler: func(msg jetstream.Msg) {
				w.handleMessage(ctx, Discrod, msg, w.discord.SendMessage)
			},
		},
		{
			name: "Telegramer",
			handler: func(msg jetstream.Msg) {
				w.handleMessage(ctx, Telegram, msg, w.telegram.SendMessage)
			},
		},
		{
			name: "OpsGeniaer",
			handler: func(msg jetstream.Msg) {
				w.handleOpsGeniaMessage(ctx, msg)
			},
		},
	}

	for _, consumer := range consumers {
		con, err := w.jetStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
			Durable:       consumer.name,
			AckPolicy:     jetstream.AckExplicitPolicy,
			MaxAckPending: 1,
		})
		if err != nil {
			return err
		}

		conCtx, consumeErr := con.Consume(consumer.handler)
		if consumeErr != nil {
			return consumeErr
		}

		g.Go(func() error {
			<-ctx.Done()
			conCtx.Stop()
			return nil
		})
	}

	return nil
}

func (w *worker) handleMessage(
	ctx context.Context, provider string,
	msg jetstream.Msg,
	sendMessageFn func(ctx context.Context, message string) error,
) {
	alert := new(models.Alert)

	if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
		w.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
		w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: provider, metrics.Status: metrics.StatusFail}).Inc()
		w.terminateMessage(msg)
		return
	}
	defer func() {
		alert = nil
	}()

	if sendErr := sendMessageFn(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
		w.log.Error(fmt.Sprintf(`Could not send finding: %v`, sendErr))
		w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: provider, metrics.Status: metrics.StatusFail}).Inc()
		w.nackMessage(msg)
		return
	}

	w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: provider, metrics.Status: metrics.StatusOk}).Inc()
	w.ackMessage(msg)
}

func (w *worker) handleOpsGeniaMessage(ctx context.Context, msg jetstream.Msg) {
	var alert models.Alert
	if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
		w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGeniea, metrics.Status: metrics.StatusFail}).Inc()
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
		w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGeniea, metrics.Status: metrics.StatusOk}).Inc()
		w.ackMessage(msg)
		return
	}

	if sendErr := w.opsGenia.SendMessage(ctx, alert.Name, alert.Description, alert.AlertID, opsGeniaPriority); sendErr != nil {
		w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGeniea, metrics.Status: metrics.StatusFail}).Inc()
		w.log.Error(fmt.Sprintf(`Could not send finding to OpsGenia: %s`, sendErr.Error()))
		w.nackMessage(msg)
		return
	}

	w.metrics.SentAlerts.With(prometheus.Labels{metrics.Channel: OpsGeniea, metrics.Status: metrics.StatusOk}).Inc()
	w.ackMessage(msg)
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

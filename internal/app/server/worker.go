package server

import (
	"context"
	"fmt"

	"github.com/nats-io/nats.go/jetstream"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/pkg/notifiler"
	"github.com/lidofinance/finding-forwarder/internal/utils/deps"
)

type worker struct {
	log       deps.Logger
	jetStream jetstream.Stream
	telegram  notifiler.Telegram
	opsGenia  notifiler.OpsGenia
	discord   notifiler.Discord
}

func NewWorker(log deps.Logger, jetStream jetstream.Stream,
	telegram notifiler.Telegram, opsGenia notifiler.OpsGenia,
	discord notifiler.Discord) *worker {
	return &worker{
		log:       log,
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
				w.handleMessage(ctx, msg, w.discord.SendMessage)
			},
		},
		{
			name: "Telegramer",
			handler: func(msg jetstream.Msg) {
				w.handleMessage(ctx, msg, w.telegram.SendMessage)
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

func (w *worker) handleMessage(ctx context.Context, msg jetstream.Msg, sendMessage func(ctx context.Context, message string) error) {
	alert := new(models.Alert)
	if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
		w.log.Errorf(`Broken message: %s`, alertErr.Error())
		w.terminateMessage(msg)
		return
	}
	defer func() {
		alert = nil
	}()

	if sendErr := sendMessage(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
		w.log.Errorf(`Could not send finding: %s`, sendErr.Error())
		w.nackMessage(msg)
		return
	}

	w.ackMessage(msg)
}

func (w *worker) handleOpsGeniaMessage(ctx context.Context, msg jetstream.Msg) {
	var alert models.Alert
	if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
		w.log.Errorf(`Broken message: %s`, alertErr.Error())
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
		w.ackMessage(msg)
		return
	}

	if sendErr := w.opsGenia.SendMessage(ctx, alert.Name, alert.Description, alert.AlertID, opsGeniaPriority); sendErr != nil {
		w.log.Errorf(`Could not send finding to OpsGenia: %s`, sendErr.Error())
		w.nackMessage(msg)
		return
	}

	w.ackMessage(msg)
}

func (w *worker) terminateMessage(msg jetstream.Msg) {
	if termErr := msg.Term(); termErr != nil {
		w.log.Errorf(`Could not term msg: %s`, termErr.Error())
	}
}

func (w *worker) nackMessage(msg jetstream.Msg) {
	if nackErr := msg.Nak(); nackErr != nil {
		w.log.Errorf(`Could not nack msg: %s`, nackErr.Error())
	}
}

func (w *worker) ackMessage(msg jetstream.Msg) {
	if ackErr := msg.Ack(); ackErr != nil {
		w.log.Errorf(`Could not ack msg: %s`, ackErr.Error())
	}
}

package server

import (
	"context"
	"fmt"
	"time"

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

const OncePerHalfSecond = 500 * time.Millisecond

// TODO right now it's common alert dispatcher
// In neaerst future we have to create to each team their own worker line OZ defender
func (w *worker) Run(ctx context.Context, g *errgroup.Group) {
	dicorder, _ := w.jetStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "Discorder",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(OncePerHalfSecond):
				_, _ = dicorder.Consume(func(msg jetstream.Msg) {
					var alert models.Alert
					if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
						w.log.Errorf(`Broken message: %s`, alertErr.Error())

						msg.Ack()
						return
					}

					if sendErr := w.discord.SendMessage(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
						w.log.Errorf(`Could not send finding to Discord: %s`, sendErr.Error())
						msg.Nak()
						return
					}

					msg.Ack()
				})
			}
		}
	})

	telegramer, _ := w.jetStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "Telegramer",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(OncePerHalfSecond):
				_, _ = telegramer.Consume(func(msg jetstream.Msg) {
					var alert models.Alert
					if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
						w.log.Errorf(`Broken message: %s`, alertErr.Error())

						msg.Ack()
						return
					}

					if sendErr := w.telegram.SendMessage(ctx, fmt.Sprintf("%s\n\n%s", alert.Name, alert.Description)); sendErr != nil {
						w.log.Errorf(`Could not send finding to Telegram: %s`, sendErr.Error())
						msg.Nak()
						return
					}

					msg.Ack()
				})
			}
		}
	})

	opsGeniaer, _ := w.jetStream.CreateOrUpdateConsumer(ctx, jetstream.ConsumerConfig{
		Durable:   "OpsGeniaer",
		AckPolicy: jetstream.AckExplicitPolicy,
	})

	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-time.After(OncePerHalfSecond):
				_, _ = opsGeniaer.Consume(func(msg jetstream.Msg) {
					var alert models.Alert
					if alertErr := alert.UnmarshalBinary(msg.Data()); alertErr != nil {
						w.log.Errorf(`Broken message: %s`, alertErr.Error())

						msg.Ack()
						return
					}

					if alert.Severity != models.AlertSeverityCRITICAL {
						msg.Ack()
						return
					}

					if sendErr := w.opsGenia.SendMessage(ctx, alert.Name, alert.Description, "P0"); sendErr != nil {
						w.log.Errorf(`Could not send finding to OpsGenia: %s`, sendErr.Error())
						msg.Nak()
						return
					}

					msg.Ack()
				})
			}
		}
	})
}

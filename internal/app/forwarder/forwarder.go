package forwarder

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
)

type worker struct {
	consumers []*consumer.Consumer

	stream jetstream.Stream
	log    *slog.Logger
}

func New(
	consumers []*consumer.Consumer,
	stream jetstream.Stream,
	log *slog.Logger,
) *worker {
	w := &worker{
		consumers: consumers,
		stream:    stream,
		log:       log,
	}

	return w
}

func (w *worker) Run(ctx context.Context, g *errgroup.Group) error {
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

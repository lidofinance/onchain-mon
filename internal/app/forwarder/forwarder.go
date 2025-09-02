package forwarder

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/nats-io/nats.go/jetstream"
	"github.com/redis/go-redis/v9"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/consumer"
)

type worker struct {
	instance  string
	rdb       *redis.Client
	consumers []*consumer.Consumer

	stream jetstream.Stream
	log    *slog.Logger

	redisConfig          *env.RedisConfig
	notificationChannels *env.NotificationChannels
}

func New(
	instance string,
	rdb *redis.Client,
	consumers []*consumer.Consumer,
	stream jetstream.Stream,
	log *slog.Logger,
	redisConfig *env.RedisConfig,
	notificationChannels *env.NotificationChannels,
) *worker {
	w := &worker{
		instance:             instance,
		rdb:                  rdb,
		consumers:            consumers,
		stream:               stream,
		log:                  log,
		redisConfig:          redisConfig,
		notificationChannels: notificationChannels,
	}

	return w
}

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

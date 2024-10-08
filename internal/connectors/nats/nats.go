package nats

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/lidofinance/onchain-mon/internal/env"
)

var (
	natsClient        *nats.Conn
	onceDefaultClient sync.Once
)

func New(cfg *env.AppConfig, logger *slog.Logger) (*nats.Conn, error) {
	var err error

	onceDefaultClient.Do(func() {
		natsClient, err = nats.Connect(cfg.NatsDefaultURL,
			nats.ReconnectWait(2*time.Second),
			nats.DisconnectErrHandler(func(_ *nats.Conn, _ error) {
				logger.Warn("Nats client got disconnected!")
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				logger.Info(fmt.Sprintf("Nats client got got reconnected to %v!", nc.ConnectedUrl()))
			}),
			nats.ClosedHandler(func(_ *nats.Conn) {
				logger.Info("Nats connection closed")
			}))
	})

	return natsClient, err
}

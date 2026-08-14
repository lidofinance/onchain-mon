package nats

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/nats-io/nats.go"

	"github.com/lidofinance/onchain-mon/internal/env"
)

const MaxMsgSize = 8 * 1024 * 1024 // 8 Mb

var (
	natsMu     sync.Mutex
	natsClient *nats.Conn
)

// New returns the shared NATS connection, dialing it on first use. A failed
// attempt is not cached: sync.Once would keep the nil client forever and drop
// the error on every later call.
func New(cfg *env.AppConfig, logger *slog.Logger) (*nats.Conn, error) {
	natsMu.Lock()
	defer natsMu.Unlock()

	if natsClient != nil && !natsClient.IsClosed() {
		return natsClient, nil
	}

	client, err := nats.Connect(cfg.NatsDefaultURL,
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(_ *nats.Conn, _ error) {
			logger.Warn("Nats client got disconnected!")
		}),
		nats.ReconnectHandler(func(nc *nats.Conn) {
			logger.Info(fmt.Sprintf("Nats client got reconnected to %v!", nc.ConnectedUrl()))
		}),
		nats.ClosedHandler(func(_ *nats.Conn) {
			logger.Info("Nats connection closed")
		}))
	if err != nil {
		return nil, fmt.Errorf("could not connect to nats at %s: %w", cfg.NatsDefaultURL, err)
	}

	natsClient = client

	return natsClient, nil
}

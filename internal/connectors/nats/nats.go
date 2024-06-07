package nats

import (
	"sync"

	"github.com/nats-io/nats.go"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

var (
	natsClient        *nats.Conn
	onceDefaultClient sync.Once
)

func New(cfg *env.AppConfig) (*nats.Conn, error) {
	var err error

	onceDefaultClient.Do(func() {
		natsClient, err = nats.Connect(cfg.NatsDefaultURL)
	})

	return natsClient, err
}

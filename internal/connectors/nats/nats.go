package nats

import (
	"fmt"
	"sync"
	"time"

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
		natsClient, err = nats.Connect(cfg.NatsDefaultURL,
			nats.ReconnectWait(2*time.Second),
			nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
				fmt.Printf("Nats client got disconnected!\n")
			}),
			nats.ReconnectHandler(func(nc *nats.Conn) {
				fmt.Printf("Nats client got got reconnected to %v!\n", nc.ConnectedUrl())
			}),
			nats.ClosedHandler(func(nc *nats.Conn) {
				fmt.Printf("Nats connection closed\n")
			}))
	})

	return natsClient, err
}

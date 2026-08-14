package redis

import (
	"context"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	writeMu     sync.Mutex
	writeClient *redis.Client
)

// Quorum bookkeeping hits Redis on every finding, so keep a few connections
// warm instead of paying the dial cost each time.
const minIdleConns = 2

// NewRedisClient returns the shared Redis client, creating it on first use.
// A failed ping does not cache a broken client: the next call retries.
func NewRedisClient(addr string, db int, log *slog.Logger, poolSize int) (*redis.Client, error) {
	writeMu.Lock()
	defer writeMu.Unlock()

	if writeClient == nil {
		writeClient = redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   db,
			// retries: for set, expire and
			MaxRetries:      5,
			MinRetryBackoff: 50 * time.Millisecond,
			MaxRetryBackoff: 500 * time.Millisecond,

			DialTimeout:  5 * time.Second,
			ReadTimeout:  3 * time.Second,
			WriteTimeout: 3 * time.Second,

			PoolSize:     poolSize,
			MinIdleConns: min(minIdleConns, poolSize),
			PoolTimeout:  1500 * time.Millisecond,

			ConnMaxIdleTime: 5 * time.Minute,

			OnConnect: func(_ context.Context, _ *redis.Conn) error {
				log.Info("redis(write): connected")
				return nil
			},
		})
	}

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if pingErr := writeClient.Ping(pingCtx).Err(); pingErr != nil {
		// Drop the client so a later call builds a fresh one instead of
		// handing back a connection that never worked.
		_ = writeClient.Close()
		writeClient = nil

		return nil, fmt.Errorf("could not ping redis at %s: %w", addr, pingErr)
	}

	return writeClient, nil
}

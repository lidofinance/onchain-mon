package redis

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	writeClientOnce sync.Once
	writeClient     *redis.Client
)

func NewRedisClient(addr string, db int, log *slog.Logger, poolSize int) (*redis.Client, error) {
	var err error

	writeClientOnce.Do(func() {
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
			MinIdleConns: poolSize / 10,
			PoolTimeout:  1500 * time.Millisecond,

			ConnMaxIdleTime: 5 * time.Minute,

			OnConnect: func(_ context.Context, _ *redis.Conn) error {
				log.Info("redis(write): connected")
				return nil
			},
		})
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pingErr := writeClient.Ping(pingCtx).Err(); pingErr != nil {
		err = pingErr
	}
	return writeClient, err
}

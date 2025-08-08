package redis

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	rdb               *redis.Client
	onceDefaultClient sync.Once
)

func NewRedisClient(ctx context.Context, addr string, db int, log *slog.Logger) (*redis.Client, error) {
	var err error

	onceDefaultClient.Do(func() {
		rdb = redis.NewClient(&redis.Options{
			Addr:            addr,
			DB:              db,
			MaxRetries:      5,
			MinRetryBackoff: 500 * time.Millisecond,
			MaxRetryBackoff: 5 * time.Second,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     3 * time.Second,
			WriteTimeout:    3 * time.Second,
			OnConnect: func(_ context.Context, _ *redis.Conn) error {
				log.Info("Redis connected")
				return nil
			},
		})

		err = rdb.Ping(ctx).Err()
	})

	return rdb, err
}

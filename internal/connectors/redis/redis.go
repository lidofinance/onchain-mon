package redis

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	rdb               *redis.Client
	onceDefaultClient sync.Once
)

func NewRedisClient(ctx context.Context, addr string, log *slog.Logger) (*redis.Client, error) {
	var err error

	onceDefaultClient.Do(func() {
		rdb = redis.NewClient(&redis.Options{
			Addr:            addr,
			DB:              1,
			MaxRetries:      5,
			MinRetryBackoff: 500 * time.Millisecond,
			MaxRetryBackoff: 5 * time.Second,
			DialTimeout:     5 * time.Second,
			ReadTimeout:     3 * time.Second,
			WriteTimeout:    3 * time.Second,
			OnConnect: func(ctx context.Context, _ *redis.Conn) error {
				log.Info("Redis connected")
				return nil
			},
		})
		rdb = rdb.WithContext(ctx)

		err = rdb.Ping(ctx).Err()
	})

	return rdb, err
}

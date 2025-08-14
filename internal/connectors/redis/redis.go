package redis

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

var (
	writeClientOnce  sync.Once
	streamClientOnce sync.Once
	writeClient      *redis.Client
	streamClient     *redis.Client
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

func NewStreamClient(addr string, db int, streamWorkers int, log *slog.Logger) (*redis.Client, error) {
	var err error

	streamClientOnce.Do(func() {
		pool := streamWorkers + 8
		if pool < 20 {
			pool = 20
		}
		streamClient = redis.NewClient(&redis.Options{
			Addr: addr,
			DB:   db,

			MaxRetries:      1,
			MinRetryBackoff: 20 * time.Millisecond,
			MaxRetryBackoff: 200 * time.Millisecond,

			DialTimeout:  3 * time.Second,
			ReadTimeout:  -1, // Nonblock for streams
			WriteTimeout: 2 * time.Second,

			PoolSize:     pool,
			MinIdleConns: streamWorkers,
			PoolTimeout:  2 * time.Second,

			ConnMaxIdleTime: 10 * time.Minute,

			OnConnect: func(_ context.Context, _ *redis.Conn) error {
				log.Info("redis(stream): connected")
				return nil
			},
		})
	})

	pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if pingErr := streamClient.Ping(pingCtx).Err(); pingErr != nil {
		err = pingErr
	}
	return streamClient, err
}

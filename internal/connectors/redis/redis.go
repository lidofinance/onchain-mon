package redis

import (
	"sync"

	"github.com/go-redis/redis"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

var (
	redisClient       *redis.Client
	onceDefaultClient sync.Once
)

func New(cfg *env.AppConfig) *redis.Client {
	onceDefaultClient.Do(func() {
		redisClient = redis.NewClient(&redis.Options{
			Addr:     cfg.RedisURL,
			Password: cfg.RedisPassword,
			DB:       cfg.RedisDB,
		})
	})

	return redisClient
}

package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-redis/redis"
	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

type App struct {
	Logger      *logrus.Logger
	Metrics     *metrics.Store
	usecase     *usecase
	RedisClient *redis.Client
}

func New(logger *logrus.Logger, promStore *metrics.Store, usecase *usecase, redisClient *redis.Client) *App {
	return &App{
		Logger:      logger,
		Metrics:     promStore,
		usecase:     usecase,
		RedisClient: redisClient,
	}
}

func (a *App) RunHTTPServer(ctx context.Context, g *errgroup.Group, appPort uint, router http.Handler) {
	server := &http.Server{
		Addr:           fmt.Sprintf(`:%d`, appPort),
		Handler:        router,
		ReadTimeout:    defaultReadTimeout,
		WriteTimeout:   defaultWriteTimeout,
		IdleTimeout:    defaultIdleTimeout,
		MaxHeaderBytes: http.DefaultMaxHeaderBytes,
	}

	g.Go(func() error {
		return server.ListenAndServe()
	})

	g.Go(func() error {
		<-ctx.Done()
		return server.Shutdown(ctx)
	})
}

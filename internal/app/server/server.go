package server

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/http/handlers/alerts"
	"github.com/lidofinance/finding-forwarder/internal/http/handlers/health"

	"github.com/sirupsen/logrus"

	"golang.org/x/sync/errgroup"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

type App struct {
	env       *env.AppConfig
	Logger    *logrus.Logger
	Metrics   *metrics.Store
	Services  *Services
	JetStream jetstream.JetStream
}

func New(config *env.AppConfig, logger *logrus.Logger, promStore *metrics.Store, services *Services, jetClient jetstream.JetStream) *App {
	return &App{
		env:       config,
		Logger:    logger,
		Metrics:   promStore,
		Services:  services,
		JetStream: jetClient,
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

func (a *App) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health.New().Handler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	alertsH := alerts.New(a.Logger, a.JetStream, a.env.NatsStreamName)

	r.Post("/alerts", alertsH.Handler)
}
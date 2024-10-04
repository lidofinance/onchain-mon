package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	slogchi "github.com/samber/slog-chi"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/http/handlers/health"
)

const (
	defaultReadTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	defaultIdleTimeout  = 60 * time.Second
)

type App struct {
	env        *env.AppConfig
	Logger     *slog.Logger
	Metrics    *metrics.Store
	JetStream  jetstream.JetStream
	natsClient *nats.Conn
}

func New(config *env.AppConfig, logger *slog.Logger,
	promStore *metrics.Store,
	jetClient jetstream.JetStream, natsClient *nats.Conn,
) *App {
	return &App{
		env:        config,
		Logger:     logger,
		Metrics:    promStore,
		JetStream:  jetClient,
		natsClient: natsClient,
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

func (a *App) RegisterHTTPRoutes(r chi.Router) {
	a.RegisterMiddleware(r)

	// alertsH := alerts.New(a.Logger, a.Metrics, a.natsClient, a.env.FortaAlertsTopic)
	// r.Post("/alerts", alertsH.Handler)

	a.RegisterInfraRoutes(r)
	a.RegisterPprofRoutes(r)
}

func (a *App) RegisterWorkerRoutes(r chi.Router) {
	a.RegisterMiddleware(r)
	a.RegisterInfraRoutes(r)
	a.RegisterPprofRoutes(r)
}

func (a *App) RegisterMiddleware(r chi.Router) {
	r.Use(slogchi.New(a.Logger))
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	const httpTimeout = 60 * time.Second
	r.Use(middleware.Timeout(httpTimeout))
}

func (a *App) RegisterPprofRoutes(r chi.Router) {
	r.Mount("/debug", middleware.Profiler())
}

func (a *App) RegisterInfraRoutes(r chi.Router) {
	r.Get("/health", health.New().Handler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)
}

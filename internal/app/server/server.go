package server

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/pprof"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/http/handlers/alerts"
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
	Services   *Services
	JetStream  jetstream.JetStream
	natsClient *nats.Conn
}

func New(config *env.AppConfig, logger *slog.Logger,
	promStore *metrics.Store, services *Services,
	jetClient jetstream.JetStream, natsClient *nats.Conn,
) *App {
	return &App{
		env:        config,
		Logger:     logger,
		Metrics:    promStore,
		Services:   services,
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

func (a *App) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	alertsH := alerts.New(a.Logger, a.natsClient, a.env.NatsStreamName)
	r.Post("/alerts", alertsH.Handler)

	r.Get("/health", health.New().Handler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	r.HandleFunc("/debug/pprof/", pprof.Index)
	r.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	r.HandleFunc("/debug/pprof/profile", pprof.Profile)
	r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	r.HandleFunc("/debug/pprof/trace", pprof.Trace)
	r.HandleFunc("/debug/pprof/{action}", pprof.Index)
}

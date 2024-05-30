package server

import (
	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/http/handlers/alerts"
	"github.com/lidofinance/finding-forwarder/internal/http/handlers/health"
)

func (a *App) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health.New().Handler)
	r.Get("/metrics", promhttp.Handler().ServeHTTP)

	alertsH := alerts.New(a.Logger)

	r.Post("/alerts", alertsH.Handler)
}

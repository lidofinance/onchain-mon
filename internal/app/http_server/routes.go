package server

import (
	"net/http"

	chi "github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	_ "github.com/prometheus/client_golang/prometheus"

	"github.com/sergeyWh1te/go-template/internal/http/handlers/health"
	userexample "github.com/sergeyWh1te/go-template/internal/http/handlers/user_example"
)

func (a *App) RegisterRoutes(r chi.Router) {
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", health.New().Handler)
	r.Method(http.MethodGet, "/metrics", promhttp.Handler())

	r.Get("/example", userexample.New(a.Logger, a.usecase.User).Handler)
}

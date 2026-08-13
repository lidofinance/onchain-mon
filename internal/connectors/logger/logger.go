package logger

import (
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	slogmulti "github.com/samber/slog-multi"
	slogsentry "github.com/samber/slog-sentry/v2"

	"github.com/lidofinance/onchain-mon/internal/env"
)

func New(cfg *env.AppConfig) (*slog.Logger, *sentry.Client, error) {
	logLevel := slog.LevelInfo

	switch strings.ToUpper(cfg.LogLevel) {
	case "DEBUG":
		logLevel = slog.LevelDebug
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	}

	var slogHandler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	if cfg.LogFormat == "json" {
		slogHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	// Sentry follows the DSN, not the environment name: an empty DSN builds a
	// client that silently discards everything, which just looks like working
	// error reporting.
	if cfg.SentryDSN == "" {
		return slog.New(slogHandler), nil, nil
	}

	hub := sentry.CurrentHub()
	client, sentryErr := sentry.NewClient(sentry.ClientOptions{
		Dsn:           cfg.SentryDSN,
		EnableTracing: false,
		Environment:   cfg.Env,
		ServerName:    cfg.Source,
	})
	if sentryErr != nil {
		return nil, nil, fmt.Errorf("could not create sentry client: %w", sentryErr)
	}

	hub.BindClient(client)

	return slog.New(
		slogmulti.Fanout(
			slogHandler,
			slogsentry.Option{
				Level: slog.LevelError,
				Hub:   hub,
			}.NewSentryHandler(),
		),
	), client, nil
}

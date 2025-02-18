package logger

import (
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
	case "INFO":
		logLevel = slog.LevelInfo
	case "WARN":
		logLevel = slog.LevelWarn
	case "ERROR":
		logLevel = slog.LevelError
	}

	var slogHandler slog.Handler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	if cfg.LogFormat == "json" {
		slogHandler = slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})
	}

	if cfg.Env != `local` {
		hub := sentry.CurrentHub()
		client, sentryErr := sentry.NewClient(sentry.ClientOptions{
			Dsn:           cfg.SentryDSN,
			EnableTracing: false,
			Environment:   cfg.Env,
			ServerName:    cfg.Source,
		})
		if sentryErr != nil {
			return nil, nil, sentryErr
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

	return slog.New(slogHandler), nil, nil
}

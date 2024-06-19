package logger

import (
	"log/slog"
	"os"
	"strings"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

func New(cfg *env.AppConfig) *slog.Logger {
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

	if cfg.LogFormat == "json" {
		return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	}

	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
}

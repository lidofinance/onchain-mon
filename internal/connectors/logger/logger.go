package logger

import (
	"os"
	"sync"

	"github.com/sirupsen/logrus"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

var (
	logger            *logrus.Logger
	onceDefaultClient sync.Once
)

func New(cfg *env.AppConfig) (*logrus.Logger, error) {
	var (
		err error
	)

	onceDefaultClient.Do(func() {
		logger = logrus.StandardLogger()

		logLevel, levelErr := logrus.ParseLevel(cfg.LogLevel)
		if levelErr != nil {
			err = levelErr
			return
		}

		logger.SetLevel(logLevel)
		logger.SetFormatter(&logrus.TextFormatter{})

		if cfg.LogFormat == "json" {
			logger.SetFormatter(&logrus.JSONFormatter{})
		}

		logger.SetOutput(os.Stdout)
	})

	return logger, err
}

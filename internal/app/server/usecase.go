package server

import (
	"net/http"
	"time"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"

	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/pkg/notifiler"
)

type Services struct {
	Telegram notifiler.Telegram
	Discord  notifiler.Discord
	OpsGenia notifiler.OpsGenia
}

func NewServices(cfg *env.AppConfig, metricsStore *metrics.Store) Services {
	transport := &http.Transport{
		MaxIdleConns:          30,
		MaxIdleConnsPerHost:   4,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	telegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID, httpClient, metricsStore)
	discord := notifiler.NewDiscord(cfg.DiscordWebHookURL, httpClient, metricsStore)
	opsGenia := notifiler.NewOpsGenia(cfg.OpsGeniaAPIKey, httpClient, metricsStore)

	return Services{
		Telegram: telegram,
		Discord:  discord,
		OpsGenia: opsGenia,
	}
}

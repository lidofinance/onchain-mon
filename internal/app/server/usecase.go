package server

import (
	"net/http"
	"time"

	"github.com/lidofinance/onchain-mon/internal/app/feeder"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

type Services struct {
	OnChainAlertsTelegram  notifiler.FindingSender
	OnChainUpdatesTelegram notifiler.FindingSender
	ErrorsTelegram         notifiler.FindingSender
	Discord                notifiler.FindingSender
	OpsGenie               notifiler.FindingSender
	ChainSrv               feeder.ChainSrv
}

func NewServices(cfg *env.AppConfig, metricsStore *metrics.Store) Services {
	transport := &http.Transport{
		MaxIdleConns:          30,
		MaxIdleConnsPerHost:   5,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	alertsTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramAlertsChatID, httpClient, metricsStore, cfg.Source)
	updatesTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramUpdatesChatID, httpClient, metricsStore, cfg.Source)
	errorsTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramErrorsChatID, httpClient, metricsStore, cfg.Source)

	discord := notifiler.NewDiscord(cfg.DiscordWebHookURL, httpClient, metricsStore, cfg.Source)
	opsGenia := notifiler.NewOpsgenie(cfg.OpsGeniaAPIKey, httpClient, metricsStore, cfg.Source)

	chainSrv := chain.NewChain(cfg.JsonRpcURL, httpClient, metricsStore)

	return Services{
		OnChainAlertsTelegram:  alertsTelegram,
		OnChainUpdatesTelegram: updatesTelegram,
		ErrorsTelegram:         errorsTelegram,
		Discord:                discord,
		OpsGenie:               opsGenia,
		ChainSrv:               chainSrv,
	}
}

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

func NewServices(cfg *env.DeliveryServicesConfig, source string, jsonRpcURL string, metricsStore *metrics.Store) Services {
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

	alertsTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramAlertsChatID, httpClient, metricsStore, source)
	updatesTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramUpdatesChatID, httpClient, metricsStore, source)
	errorsTelegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramErrorsChatID, httpClient, metricsStore, source)

	discord := notifiler.NewDiscord(cfg.DiscordWebHookURL, httpClient, metricsStore, source)
	opsGenie := notifiler.NewOpsgenie(cfg.OpsGenieAPIKey, httpClient, metricsStore, source)

	chainSrv := chain.NewChain(jsonRpcURL, httpClient, metricsStore)

	return Services{
		OnChainAlertsTelegram:  alertsTelegram,
		OnChainUpdatesTelegram: updatesTelegram,
		ErrorsTelegram:         errorsTelegram,
		Discord:                discord,
		OpsGenie:               opsGenie,
		ChainSrv:               chainSrv,
	}
}

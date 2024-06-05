package server

import (
	"net/http"

	"github.com/lidofinance/finding-forwarder/internal/pkg/notifiler"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

type Services struct {
	Telegram notifiler.Telegram
	Discord  notifiler.Discord
	OpsGenia notifiler.OpsGenia
}

func NewServices(cfg *env.AppConfig) Services {
	httpClient := &http.Client{}
	telegram := notifiler.NewTelegram(cfg.TelegramBotToken, cfg.TelegramChatID, httpClient)
	discord := notifiler.NewDiscord(cfg.DiscordWebHookURL, httpClient)
	opsGenia := notifiler.NewOpsGenia(cfg.OpsGeniaAPIKey, httpClient)

	return Services{
		Telegram: telegram,
		Discord:  discord,
		OpsGenia: opsGenia,
	}
}

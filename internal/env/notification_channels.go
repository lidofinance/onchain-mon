package env

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

type NotificationChannels struct {
	TelegramChannels map[string]*notifiler.Telegram
	DiscordChannels  map[string]*notifiler.Discord
	OpsGenieChannels map[string]*notifiler.OpsGenie
}

func NewNotificationChannels(
	log *slog.Logger,
	cfg *NotificationConfig,
	httpClient *http.Client,
	metricsStore *metrics.Store,
	blockExplorer string,
	redisConfig *RedisConfig,
) (*NotificationChannels, error) {
	channels := &NotificationChannels{
		TelegramChannels: make(map[string]*notifiler.Telegram),
		DiscordChannels:  make(map[string]*notifiler.Discord),
		OpsGenieChannels: make(map[string]*notifiler.OpsGenie),
	}

	for _, tgChannel := range cfg.TelegramChannels {
		channels.TelegramChannels[tgChannel.ID] = notifiler.NewTelegram(
			tgChannel.BotToken,
			tgChannel.ChatID,
			httpClient,
			metricsStore,
			blockExplorer,
			tgChannel.ID,
			redisConfig.TelegramStreamName,
			redisConfig.TelegramConsumerGroupName,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", tgChannel.ID, tgChannel.Description))
	}

	for _, discordChannel := range cfg.DiscordChannels {
		channels.DiscordChannels[discordChannel.ID] = notifiler.NewDiscord(
			discordChannel.WebhookURL,
			httpClient,
			metricsStore,
			blockExplorer,
			discordChannel.ID,
			redisConfig.DiscordStreamName,
			redisConfig.DiscordConsumerGroupName,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", discordChannel.ID, discordChannel.Description))
	}

	for _, opsGenieChannel := range cfg.OpsGenieChannels {
		channels.OpsGenieChannels[opsGenieChannel.ID] = notifiler.NewOpsgenie(
			opsGenieChannel.APIKey,
			httpClient,
			metricsStore,
			blockExplorer,
			opsGenieChannel.ID,
			redisConfig.OpsGenieStreamName,
			redisConfig.OpsGeniaConsumerGroupName,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", opsGenieChannel.ID, opsGenieChannel.Description))
	}

	return channels, nil
}

func (n *NotificationChannels) Count() int {
	return len(n.TelegramChannels) + len(n.DiscordChannels) + len(n.OpsGenieChannels)
}

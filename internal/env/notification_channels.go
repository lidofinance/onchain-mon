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
	SlackChannels    map[string]*notifiler.Slack
}

func NewNotificationChannels(
	log *slog.Logger,
	cfg *NotificationConfig,
	httpClient *http.Client,
	metricsStore *metrics.Store,
	blockExplorer, source string,
	env string,
) (*NotificationChannels, error) {
	channels := &NotificationChannels{
		TelegramChannels: make(map[string]*notifiler.Telegram),
		DiscordChannels:  make(map[string]*notifiler.Discord),
		OpsGenieChannels: make(map[string]*notifiler.OpsGenie),
		SlackChannels:    make(map[string]*notifiler.Slack),
	}

	for _, tgChannel := range cfg.TelegramChannels {
		channels.TelegramChannels[tgChannel.ID] = notifiler.NewTelegram(
			tgChannel.BotToken,
			tgChannel.ChatID,
			httpClient,
			metricsStore,
			source,
			blockExplorer,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", tgChannel.ID, tgChannel.Description))
	}

	for _, discordChannel := range cfg.DiscordChannels {
		channels.DiscordChannels[discordChannel.ID] = notifiler.NewDiscord(
			discordChannel.WebhookURL,
			httpClient,
			metricsStore,
			source,
			blockExplorer,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", discordChannel.ID, discordChannel.Description))
	}

	for _, opsGenieChannel := range cfg.OpsGenieChannels {
		channels.OpsGenieChannels[opsGenieChannel.ID] = notifiler.NewOpsgenie(
			opsGenieChannel.APIKey,
			httpClient,
			metricsStore,
			source,
			blockExplorer,
			env,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", opsGenieChannel.ID, opsGenieChannel.Description))
	}

	for _, slackChannel := range cfg.SlackChannels {
		channels.SlackChannels[slackChannel.ID] = notifiler.NewSlack(
			slackChannel.WebhookURL,
			httpClient,
			metricsStore,
			source,
			blockExplorer,
		)
		log.Info(fmt.Sprintf("Initialized %s channel: %s", slackChannel.ID, slackChannel.Description))
	}

	return channels, nil
}

func (n *NotificationChannels) Count() int {
	return len(n.TelegramChannels) + len(n.DiscordChannels) + len(n.OpsGenieChannels) + len(n.SlackChannels)
}

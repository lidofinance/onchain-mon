package env

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/viper"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

const (
	ChannelTypeTelegram = "Telegram"
	ChannelTypeDiscord  = "Discord"
	ChannelTypeOpsGenie = "OpsGenie"
)

type SeverityLevel struct {
	ID string `mapstructure:"id"`
}

type TelegramChannel struct {
	ID          string `mapstructure:"id"`
	Description string `mapstructure:"description"`
	BotToken    string `mapstructure:"bot_token"`
	ChatID      string `mapstructure:"chat_id"`
}

type DiscordChannel struct {
	ID          string `mapstructure:"id"`
	Description string `mapstructure:"description"`
	WebhookURL  string `mapstructure:"webhook_url"`
}

type OpsGenieChannel struct {
	ID          string `mapstructure:"id"`
	Description string `mapstructure:"description"`
	APIKey      string `mapstructure:"api_key"`
}

type Consumer struct {
	ConsumerName string   `mapstructure:"consumerName"`
	Type         string   `mapstructure:"type"`
	ChannelID    string   `mapstructure:"channel_id"`
	Severities   []string `mapstructure:"severities"`
	ByQuorum     bool     `mapstructure:"by_quorum"`
	Subjects     []string `mapstructure:"subjects"`
	SeveritySet  registry.FindingMapping
}

type NotificationConfig struct {
	SeverityLevels   []SeverityLevel   `mapstructure:"severity_levels"`
	TelegramChannels []TelegramChannel `mapstructure:"telegram_channels"`
	DiscordChannels  []DiscordChannel  `mapstructure:"discord_channels"`
	OpsGenieChannels []OpsGenieChannel `mapstructure:"opsgenie_channels"`
	Consumers        []*Consumer       `mapstructure:"consumers"`
}

func ReadNotificationConfig(env, configPath string) (*NotificationConfig, error) {
	v := viper.New()

	if env != `local` {
		configPath = `/etc/forwarder/notification.yaml`
	}

	if _, err := os.Stat(configPath); err != nil {
		return nil, err
	}

	v.SetConfigName(filepath.Base(configPath))
	v.SetConfigType("yaml")
	v.AddConfigPath(filepath.Dir(configPath))

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("error reading config file, %s", err)
	}

	var configData NotificationConfig

	if err := v.Unmarshal(&configData); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %v", err)
	}

	if err := ValidateConfig(&configData); err != nil {
		return nil, err
	}

	return &configData, nil
}

// ValidateConfig performs semantic and logical validation of the configuration
func ValidateConfig(cfg *NotificationConfig) error {
	consumerNames := make(map[string]struct {
		ChannelID string
	})

	for _, consumer := range cfg.Consumers {
		if existingChannel, exists := consumerNames[consumer.ConsumerName]; exists {
			return fmt.Errorf("consumerName '%s' is duplicated (channel '%s') and (channel '%s')",
				consumer.ConsumerName, consumer.ChannelID, existingChannel.ChannelID)
		}
		consumerNames[consumer.ConsumerName] = struct {
			ChannelID string
		}{
			ChannelID: consumer.ChannelID,
		}
	}

	telegramChannels := make(map[string]bool)
	for _, channel := range cfg.TelegramChannels {
		telegramChannels[channel.ID] = true
	}

	discordChannels := make(map[string]bool)
	for _, channel := range cfg.DiscordChannels {
		discordChannels[channel.ID] = true
	}

	opsgenieChannels := make(map[string]bool)
	for _, channel := range cfg.OpsGenieChannels {
		opsgenieChannels[channel.ID] = true
	}

	for _, consumer := range cfg.Consumers {
		switch consumer.Type {
		case ChannelTypeTelegram:
			if _, exists := telegramChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown Telegram channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		case ChannelTypeDiscord:
			if _, exists := discordChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown Discord channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		case ChannelTypeOpsGenie:
			if _, exists := opsgenieChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown OpsGenie channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		default:
			return fmt.Errorf("consumer '%s' has an unknown type '%s'", consumer.ConsumerName, consumer.Type)
		}
	}

	validSeverities := make(registry.FindingMapping)
	for _, severity := range cfg.SeverityLevels {
		validSeverities[databus.Severity(severity.ID)] = true
	}

	for _, consumer := range cfg.Consumers {
		severitySet := make(registry.FindingMapping)
		for _, severity := range consumer.Severities {
			if _, exists := validSeverities[databus.Severity(severity)]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown severity level '%s'", consumer.ConsumerName, severity)
			}
			severitySet[databus.Severity(severity)] = true
		}

		consumer.SeveritySet = severitySet
	}

	for _, consumer := range cfg.Consumers {
		if len(consumer.Subjects) == 0 {
			return fmt.Errorf("consumer '%s' does not have any NATS subjects configured", consumer.ConsumerName)
		}
	}

	return nil
}

func CollectNatsSubjects(cfg *NotificationConfig) []string {
	natsSubjectsMap := make(map[string]bool)

	for _, consumer := range cfg.Consumers {
		for _, subject := range consumer.Subjects {
			natsSubjectsMap[subject] = true
		}
	}

	out := make([]string, 0, len(natsSubjectsMap))
	for subject := range natsSubjectsMap {
		out = append(out, subject)
	}

	sort.Strings(out)
	return out
}

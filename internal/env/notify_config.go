package env

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

// SubjectParts is the minimum number of dot-separated parts in a findings
// subject: findings.<team>.<bot>.
const SubjectParts = 3

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

type SlackChannel struct {
	ID          string `mapstructure:"id"`
	Description string `mapstructure:"description"`
	WebhookURL  string `mapstructure:"webhook_url"`
}

type Consumer struct {
	ConsumerName     string                       `mapstructure:"consumerName"`
	Type             registry.NotificationChannel `mapstructure:"type"`
	ChannelID        string                       `mapstructure:"channel_id"`
	Severities       []string                     `mapstructure:"severities"`
	ByQuorum         bool                         `mapstructure:"by_quorum"`
	Subjects         []string                     `mapstructure:"subjects"`
	Filter           []string                     `mapstructure:"filter"`
	SeveritySet      registry.FindingMapping
	FindingFilterMap registry.FindingFilterMap
}

type NotificationConfig struct {
	SeverityLevels   []SeverityLevel   `mapstructure:"severity_levels"`
	TelegramChannels []TelegramChannel `mapstructure:"telegram_channels"`
	DiscordChannels  []DiscordChannel  `mapstructure:"discord_channels"`
	OpsGenieChannels []OpsGenieChannel `mapstructure:"opsgenie_channels"`
	SlackChannels    []SlackChannel    `mapstructure:"slack_channels"`
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
		return nil, fmt.Errorf("error reading config file, %w", err)
	}

	var configData NotificationConfig

	if err := v.Unmarshal(&configData); err != nil {
		return nil, fmt.Errorf("unable to decode into struct, %w", err)
	}

	if err := ValidateConfig(&configData); err != nil {
		return nil, err
	}

	return &configData, nil
}

// ValidateConfig performs semantic and logical validation of the configuration
func ValidateConfig(cfg *NotificationConfig) error {
	// A forwarder with no consumers starts up and quietly forwards nothing,
	// which looks like a healthy service — fail on the config instead.
	if len(cfg.Consumers) == 0 {
		return errors.New("notification config has no consumers")
	}

	if len(cfg.SeverityLevels) == 0 {
		return errors.New("notification config has no severity_levels")
	}

	if err := validateUniqueConsumerNames(cfg); err != nil {
		return err
	}

	if err := validateChannelRefs(cfg); err != nil {
		return err
	}

	if err := validateSeverities(cfg); err != nil {
		return err
	}

	return validateSubjects(cfg)
}

// validateSubjects checks the subject format NewConsumers relies on and makes
// sure no two consumers end up sharing a JetStream durable name.
func validateSubjects(cfg *NotificationConfig) error {
	durableNames := make(map[string]string, len(cfg.Consumers))

	for _, consumer := range cfg.Consumers {
		if len(consumer.Subjects) == 0 {
			return fmt.Errorf("consumer '%s' does not have any NATS subjects configured", consumer.ConsumerName)
		}

		for _, subject := range consumer.Subjects {
			// NewConsumers splits on "." and takes parts[1] and parts[2]; a
			// shorter subject makes the forwarder fail on startup instead.
			parts := strings.Split(subject, ".")
			if len(parts) < SubjectParts {
				return fmt.Errorf("consumer '%s' has an invalid subject '%s', expected findings.<team>.<bot>",
					consumer.ConsumerName, subject)
			}

			for i, part := range parts[:SubjectParts] {
				if part == "" {
					return fmt.Errorf("consumer '%s' has an empty part %d in subject '%s'",
						consumer.ConsumerName, i+1, subject)
				}
			}

			// Durable names are built as <team>_<consumerName>_<bot>, so two
			// different consumers can still collide and silently share one
			// JetStream consumer.
			durable := fmt.Sprintf("%s_%s_%s", parts[1], consumer.ConsumerName, parts[2])
			if owner, exists := durableNames[durable]; exists {
				return fmt.Errorf("consumers '%s' and '%s' both map to the NATS durable name '%s'",
					owner, consumer.ConsumerName, durable)
			}
			durableNames[durable] = consumer.ConsumerName
		}
	}

	return nil
}

func validateUniqueConsumerNames(cfg *NotificationConfig) error {
	consumerNames := make(map[string]struct {
		ChannelID string
	})

	for _, consumer := range cfg.Consumers {
		// The name ends up inside the NATS durable name, so an empty one yields
		// a malformed identifier like "team__bot".
		if consumer.ConsumerName == "" {
			return fmt.Errorf("consumer referencing channel '%s' has an empty consumerName", consumer.ChannelID)
		}

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

	return nil
}

func validateChannelRefs(cfg *NotificationConfig) error {
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

	slackChannels := make(map[string]bool)
	for _, channel := range cfg.SlackChannels {
		slackChannels[channel.ID] = true
	}

	for _, consumer := range cfg.Consumers {
		switch consumer.Type {
		case registry.Telegram:
			if _, exists := telegramChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown Telegram channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		case registry.Discord:
			if _, exists := discordChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown Discord channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		case registry.OpsGenie:
			if _, exists := opsgenieChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown OpsGenie channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		case registry.Slack:
			if _, exists := slackChannels[consumer.ChannelID]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown Slack channel '%s'", consumer.ConsumerName, consumer.ChannelID)
			}
		default:
			return fmt.Errorf("consumer '%s' has an unknown type '%s'", consumer.ConsumerName, consumer.Type)
		}
	}

	return nil
}

func validateSeverities(cfg *NotificationConfig) error {
	validSeverities := make(registry.FindingMapping)
	for _, severity := range cfg.SeverityLevels {
		validSeverities[databus.Severity(severity.ID)] = true
	}

	for _, consumer := range cfg.Consumers {
		// An empty set makes the handler ack every finding without sending it,
		// so the consumer looks alive while dropping everything.
		if len(consumer.Severities) == 0 {
			return fmt.Errorf("consumer '%s' does not have any severities configured", consumer.ConsumerName)
		}

		severitySet := make(registry.FindingMapping)
		findingFilter := make(registry.FindingFilterMap)

		for _, severity := range consumer.Severities {
			if _, exists := validSeverities[databus.Severity(severity)]; !exists {
				return fmt.Errorf("consumer '%s' references an unknown severity level '%s'", consumer.ConsumerName, severity)
			}
			severitySet[databus.Severity(severity)] = true
		}

		for _, alertID := range consumer.Filter {
			findingFilter[alertID] = true
		}

		consumer.SeveritySet = severitySet
		consumer.FindingFilterMap = findingFilter
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

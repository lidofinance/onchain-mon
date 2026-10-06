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
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

// SubjectParts is the exact number of dot-separated parts in a findings
// subject: findings.<team>.<bot>.
const SubjectParts = 3

// SubjectPrefix is the first part of every findings subject the bots publish to.
const SubjectPrefix = "findings"

// forbiddenSubjectChars are the NATS wildcards plus forbidden characters
const forbiddenSubjectChars = "*> \t\r\n/\\"

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
			// The stream and the durable consumer filter on the raw subject
			// while NewConsumers derives the durable name from parts[1] and
			// parts[2] only. A wrong prefix or an extra part therefore builds a
			// healthy-looking consumer subscribed to a subject nobody publishes
			// to, so require the canonical shape instead.
			parts := strings.Split(subject, ".")
			if len(parts) != SubjectParts || parts[0] != SubjectPrefix {
				return fmt.Errorf("consumer '%s' has an invalid subject '%s', expected %s.<team>.<bot>",
					consumer.ConsumerName, subject, SubjectPrefix)
			}

			for i, part := range parts {
				if part == "" {
					return fmt.Errorf("consumer '%s' has an empty part %d in subject '%s'",
						consumer.ConsumerName, i+1, subject)
				}

				if strings.ContainsAny(part, forbiddenSubjectChars) {
					return fmt.Errorf("consumer '%s' has a forbidden character in part %d of subject '%s' (wildcards are not supported)",
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

type channelDecl interface {
	id() string
	validate() error
}

func (c TelegramChannel) id() string { return c.ID }

func (c TelegramChannel) validate() error {
	if c.BotToken == "" {
		return errors.New("has an empty bot_token")
	}

	if c.ChatID == "" {
		return errors.New("has an empty chat_id")
	}

	return nil
}

func (c DiscordChannel) id() string { return c.ID }

func (c DiscordChannel) validate() error {
	if c.WebhookURL == "" {
		return errors.New("has an empty webhook_url")
	}

	return nil
}

func (c OpsGenieChannel) id() string { return c.ID }

func (c OpsGenieChannel) validate() error {
	if c.APIKey == "" {
		return errors.New("has an empty api_key")
	}

	return nil
}

func (c SlackChannel) id() string { return c.ID }

func (c SlackChannel) validate() error {
	if c.WebhookURL == "" {
		return errors.New("has an empty webhook_url")
	}

	return nil
}

// collectChannelIDs indexes the declarations of one channel type by id and rejects empty or duplicated ones
func collectChannelIDs[T channelDecl](kind string, channels []T) (map[string]struct{}, error) {
	firstSeen := make(map[string]int, len(channels))

	for i, channel := range channels {
		channelID := channel.id()
		if channelID == "" {
			return nil, fmt.Errorf("%s_channels[%d] has an empty id", kind, i)
		}

		if first, exists := firstSeen[channelID]; exists {
			return nil, fmt.Errorf("%s_channels[%d] and %s_channels[%d] both declare the id '%s'",
				kind, first, kind, i, channelID)
		}

		if err := channel.validate(); err != nil {
			return nil, fmt.Errorf("%s_channels[%d] '%s' %w", kind, i, channelID, err)
		}

		firstSeen[channelID] = i
	}

	ids := make(map[string]struct{}, len(firstSeen))
	for channelID := range firstSeen {
		ids[channelID] = struct{}{}
	}

	return ids, nil
}

func validateChannelRefs(cfg *NotificationConfig) error {
	telegramChannels, err := collectChannelIDs("telegram", cfg.TelegramChannels)
	if err != nil {
		return err
	}

	discordChannels, err := collectChannelIDs("discord", cfg.DiscordChannels)
	if err != nil {
		return err
	}

	opsgenieChannels, err := collectChannelIDs("opsgenie", cfg.OpsGenieChannels)
	if err != nil {
		return err
	}

	slackChannels, err := collectChannelIDs("slack", cfg.SlackChannels)
	if err != nil {
		return err
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

func collectGlobalSeverities(cfg *NotificationConfig) (registry.FindingMapping, error) {
	validSeverities := make(registry.FindingMapping, len(cfg.SeverityLevels))

	for i, severity := range cfg.SeverityLevels {
		if severity.ID == "" {
			return nil, fmt.Errorf("severity_levels[%d] has an empty id", i)
		}

		if !registry.CanonicalSeverities[databus.Severity(severity.ID)] {
			return nil, fmt.Errorf("severity_levels[%d] declares an unknown severity '%s', expected one of: %s",
				i, severity.ID, registry.CanonicalSeverityList())
		}

		if validSeverities[databus.Severity(severity.ID)] {
			return nil, fmt.Errorf("severity_levels[%d] duplicates severity '%s'", i, severity.ID)
		}

		validSeverities[databus.Severity(severity.ID)] = true
	}

	return validSeverities, nil
}

// validateChannelSeverities rejects severities the channel cannot actually deliver.
func validateChannelSeverities(consumer *Consumer) error {
	if consumer.Type != registry.OpsGenie {
		return nil
	}

	for _, severity := range consumer.Severities {
		if notifiler.OpsGeniePriority(databus.Severity(severity)) == "" {
			return fmt.Errorf("consumer '%s' cannot deliver severity '%s' to OpsGenie, supported: %s",
				consumer.ConsumerName, severity, notifiler.OpsGenieSeverityList())
		}
	}

	return nil
}

func validateSeverities(cfg *NotificationConfig) error {
	validSeverities, err := collectGlobalSeverities(cfg)
	if err != nil {
		return err
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

		if err := validateChannelSeverities(consumer); err != nil {
			return err
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

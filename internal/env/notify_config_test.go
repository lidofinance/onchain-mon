package env

import (
	"strings"
	"testing"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

func telegramChannel(id string) TelegramChannel {
	return TelegramChannel{ID: id, BotToken: "bot-token", ChatID: "chat-id"}
}

func discordChannel(id string) DiscordChannel {
	return DiscordChannel{ID: id, WebhookURL: "https://discord.example/hook"}
}

func opsGenieChannel(id string) OpsGenieChannel {
	return OpsGenieChannel{ID: id, APIKey: "api-key"}
}

func slackChannel(id string) SlackChannel {
	return SlackChannel{ID: id, WebhookURL: "https://slack.example/hook"}
}

func validConfig() *NotificationConfig {
	return &NotificationConfig{
		SeverityLevels:   []SeverityLevel{{ID: "Critical"}, {ID: "High"}},
		TelegramChannels: []TelegramChannel{telegramChannel("tg1")},
		Consumers: []*Consumer{{
			ConsumerName: "alerts",
			Type:         registry.Telegram,
			ChannelID:    "tg1",
			Severities:   []string{"Critical"},
			Subjects:     []string{"findings.alpha.watcher"},
		}},
	}
}

func Test_valid_config_passes(t *testing.T) {
	cfg := validConfig()
	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Validation also fills the derived lookup maps the handler relies on.
	c := cfg.Consumers[0]
	if !c.SeveritySet[databus.Severity("Critical")] {
		t.Error("SeveritySet was not populated")
	}
	if c.FindingFilterMap == nil {
		t.Error("FindingFilterMap must be initialized even without a filter")
	}
}

func Test_config_is_rejected_when(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*NotificationConfig)
		wantErr string
	}{
		{
			// A forwarder with no consumers looks healthy and forwards nothing.
			name:    "no_consumers",
			mutate:  func(c *NotificationConfig) { c.Consumers = nil },
			wantErr: "no consumers",
		},
		{
			name:    "no_severity_levels",
			mutate:  func(c *NotificationConfig) { c.SeverityLevels = nil },
			wantErr: "no severity_levels",
		},
		{
			// An empty severity set acks every finding without sending it.
			name:    "consumer_without_severities",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Severities = nil },
			wantErr: "does not have any severities",
		},
		{
			name:    "consumer_without_subjects",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = nil },
			wantErr: "does not have any NATS subjects",
		},
		{
			name:    "unknown_channel",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].ChannelID = "nope" },
			wantErr: "unknown Telegram channel",
		},
		{
			name:    "unknown_severity",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Severities = []string{"Bogus"} },
			wantErr: "unknown severity level",
		},
		{
			name:    "unknown_type",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Type = "carrier-pigeon" },
			wantErr: "unknown type",
		},
		{
			name: "duplicated_consumer_name",
			mutate: func(c *NotificationConfig) {
				dup := *c.Consumers[0]
				c.Consumers = append(c.Consumers, &dup)
			},
			wantErr: "is duplicated",
		},
		{
			// The name is part of the durable identifier, so it cannot be blank.
			name:    "empty_consumer_name",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].ConsumerName = "" },
			wantErr: "empty consumerName",
		},
		{
			// NewConsumers needs findings.<team>.<bot>; anything shorter used to
			// pass validation and crash the forwarder on startup instead.
			name:    "subject_with_too_few_parts",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings.team"} },
			wantErr: "invalid subject",
		},
		{
			name:    "subject_with_empty_part",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings..bot"} },
			wantErr: "empty part",
		},
		{
			name:    "subject_with_wrong_prefix",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"finding.team.bot"} },
			wantErr: "invalid subject",
		},
		{
			name:    "subject_with_extra_part",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings.team.bot.alerts"} },
			wantErr: "invalid subject",
		},
		{
			name:    "subject_with_wildcard",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings.team.*"} },
			wantErr: "forbidden character",
		},
		{
			name:    "subject_with_full_wildcard",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings.team.>"} },
			wantErr: "forbidden character",
		},
		{
			name:    "subject_with_whitespace",
			mutate:  func(c *NotificationConfig) { c.Consumers[0].Subjects = []string{"findings.team.my bot"} },
			wantErr: "forbidden character",
		},
		{
			// "a_b" + findings.x.y and "b" + findings.x_a.y both build the
			// durable name x_a_b_y, so the two would share one NATS consumer.
			name: "colliding_durable_names",
			mutate: func(c *NotificationConfig) {
				c.Consumers[0].ConsumerName = "a_b"
				c.Consumers[0].Subjects = []string{"findings.x.y"}
				c.Consumers = append(c.Consumers, &Consumer{
					ConsumerName: "b",
					Type:         registry.Telegram,
					ChannelID:    "tg1",
					Severities:   []string{"Critical"},
					Subjects:     []string{"findings.x_a.y"},
				})
			},
			wantErr: "durable name",
		},
		{
			name: "global_severity_typo_matched_by_consumer",
			mutate: func(c *NotificationConfig) {
				c.SeverityLevels = []SeverityLevel{{ID: "Critcal"}}
				c.Consumers[0].Severities = []string{"Critcal"}
			},
			wantErr: "unknown severity 'Critcal'",
		},
		{
			name: "empty_global_severity_id",
			mutate: func(c *NotificationConfig) {
				c.SeverityLevels = []SeverityLevel{{ID: ""}}
				c.Consumers[0].Severities = []string{""}
			},
			wantErr: "empty id",
		},
		{
			name: "duplicated_global_severity",
			mutate: func(c *NotificationConfig) {
				c.SeverityLevels = []SeverityLevel{{ID: "Critical"}, {ID: "Critical"}}
			},
			wantErr: "duplicates severity 'Critical'",
		},
		{
			name: "opsgenie_consumer_with_undeliverable_severity",
			mutate: func(c *NotificationConfig) {
				c.SeverityLevels = append(c.SeverityLevels, SeverityLevel{ID: "Medium"})
				c.OpsGenieChannels = []OpsGenieChannel{opsGenieChannel("og1")}
				c.Consumers[0].Type = registry.OpsGenie
				c.Consumers[0].ChannelID = "og1"
				c.Consumers[0].Severities = []string{"Critical", "Medium"}
			},
			wantErr: "cannot deliver severity 'Medium' to OpsGenie",
		},
		{
			name: "duplicated_telegram_channel_id",
			mutate: func(c *NotificationConfig) {
				c.TelegramChannels = append(c.TelegramChannels, TelegramChannel{ID: "tg1", BotToken: "bot-token", ChatID: "someone-else"})
			},
			wantErr: "telegram_channels[0] and telegram_channels[1] both declare the id 'tg1'",
		},
		{
			name: "duplicated_discord_channel_id",
			mutate: func(c *NotificationConfig) {
				c.DiscordChannels = []DiscordChannel{discordChannel("dc1"), discordChannel("dc1")}
			},
			wantErr: "discord_channels[0] and discord_channels[1] both declare the id 'dc1'",
		},
		{
			name: "duplicated_opsgenie_channel_id",
			mutate: func(c *NotificationConfig) {
				c.OpsGenieChannels = []OpsGenieChannel{opsGenieChannel("og-dup"), opsGenieChannel("og-dup")}
			},
			wantErr: "opsgenie_channels[0] and opsgenie_channels[1] both declare the id 'og-dup'",
		},
		{
			name: "duplicated_slack_channel_id",
			mutate: func(c *NotificationConfig) {
				c.SlackChannels = []SlackChannel{slackChannel("sl1"), slackChannel("sl1")}
			},
			wantErr: "slack_channels[0] and slack_channels[1] both declare the id 'sl1'",
		},
		{
			name: "telegram_channel_without_bot_token",
			mutate: func(c *NotificationConfig) {
				c.TelegramChannels = []TelegramChannel{{ID: "tg1", ChatID: "chat-id"}}
			},
			wantErr: "telegram_channels[0] 'tg1' has an empty bot_token",
		},
		{
			name: "telegram_channel_without_chat_id",
			mutate: func(c *NotificationConfig) {
				c.TelegramChannels = []TelegramChannel{{ID: "tg1", BotToken: "bot-token"}}
			},
			wantErr: "telegram_channels[0] 'tg1' has an empty chat_id",
		},
		{
			name: "discord_channel_without_webhook_url",
			mutate: func(c *NotificationConfig) {
				c.DiscordChannels = []DiscordChannel{{ID: "dc1"}}
			},
			wantErr: "discord_channels[0] 'dc1' has an empty webhook_url",
		},
		{
			name: "opsgenie_channel_without_api_key",
			mutate: func(c *NotificationConfig) {
				c.OpsGenieChannels = []OpsGenieChannel{{ID: "og1"}}
			},
			wantErr: "opsgenie_channels[0] 'og1' has an empty api_key",
		},
		{
			name: "slack_channel_without_webhook_url",
			mutate: func(c *NotificationConfig) {
				c.SlackChannels = []SlackChannel{{ID: "sl1"}}
			},
			wantErr: "slack_channels[0] 'sl1' has an empty webhook_url",
		},
		{
			name: "empty_channel_id",
			mutate: func(c *NotificationConfig) {
				c.TelegramChannels = []TelegramChannel{{ID: ""}}
				c.Consumers[0].ChannelID = ""
			},
			wantErr: "telegram_channels[0] has an empty id",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := validConfig()
			tt.mutate(cfg)

			err := ValidateConfig(cfg)
			if err == nil {
				t.Fatalf("expected an error mentioning %q", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("got %q, want it to mention %q", err, tt.wantErr)
			}
		})
	}
}

func Test_collect_nats_subjects_is_deduped_and_sorted(t *testing.T) {
	cfg := validConfig()
	cfg.Consumers[0].Subjects = []string{"findings.b.two", "findings.a.one", "findings.b.two"}

	got := CollectNatsSubjects(cfg)

	want := []string{"findings.a.one", "findings.b.two"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func Test_every_canonical_severity_is_accepted(t *testing.T) {
	levels := make([]SeverityLevel, 0, len(registry.CanonicalSeverities))
	severities := make([]string, 0, len(registry.CanonicalSeverities))
	for severity := range registry.CanonicalSeverities {
		levels = append(levels, SeverityLevel{ID: string(severity)})
		severities = append(severities, string(severity))
	}

	cfg := validConfig()
	cfg.SeverityLevels = levels
	cfg.Consumers[0].Severities = severities

	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for severity := range registry.CanonicalSeverities {
		if !cfg.Consumers[0].SeveritySet[severity] {
			t.Errorf("severity %q is missing from SeveritySet", severity)
		}
	}
}

func Test_opsgenie_consumer_with_pageable_severities_passes(t *testing.T) {
	cfg := validConfig()
	cfg.OpsGenieChannels = []OpsGenieChannel{opsGenieChannel("og1")}
	cfg.Consumers[0].Type = registry.OpsGenie
	cfg.Consumers[0].ChannelID = "og1"
	cfg.Consumers[0].Severities = []string{"High", "Critical"}

	if err := ValidateConfig(cfg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// Only OpsGenie restricts severities, so the other channels must keep accepting
// every canonical one.
func Test_non_opsgenie_consumers_accept_every_severity(t *testing.T) {
	for _, channel := range []registry.NotificationChannel{registry.Telegram, registry.Discord, registry.Slack} {
		cfg := validConfig()
		cfg.DiscordChannels = []DiscordChannel{discordChannel("ch1")}
		cfg.SlackChannels = []SlackChannel{slackChannel("ch1")}
		cfg.TelegramChannels = []TelegramChannel{telegramChannel("ch1")}
		cfg.Consumers[0].Type = channel
		cfg.Consumers[0].ChannelID = "ch1"

		levels := make([]SeverityLevel, 0, len(registry.CanonicalSeverities))
		severities := make([]string, 0, len(registry.CanonicalSeverities))
		for severity := range registry.CanonicalSeverities {
			levels = append(levels, SeverityLevel{ID: string(severity)})
			severities = append(severities, string(severity))
		}
		cfg.SeverityLevels = levels
		cfg.Consumers[0].Severities = severities

		if err := ValidateConfig(cfg); err != nil {
			t.Errorf("%s: unexpected error: %v", channel, err)
		}
	}
}

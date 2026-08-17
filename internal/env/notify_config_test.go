package env

import (
	"strings"
	"testing"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

func validConfig() *NotificationConfig {
	return &NotificationConfig{
		SeverityLevels:   []SeverityLevel{{ID: "Critical"}, {ID: "High"}},
		TelegramChannels: []TelegramChannel{{ID: "tg1"}},
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

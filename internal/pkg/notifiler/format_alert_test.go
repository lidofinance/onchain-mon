package notifiler_test

import (
	"strings"
	"testing"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

func TestFormatAlert(t *testing.T) {
	tests := []struct {
		name          string
		alert         *databus.FindingDtoJson
		source        string
		blockExplorer string
		wantContains  []string
		wantAbsent    []string
	}{
		{
			name: "basic alert with all fields",
			alert: &databus.FindingDtoJson{
				Name:           "Test Alert",
				Description:    "Something happened",
				Severity:       databus.SeverityHigh,
				AlertId:        "TEST-ALERT-1",
				BotName:        "test-bot",
				Team:           "test-team",
				UniqueKey:      "abc123",
				BlockNumber:    new(100),
				BlockTimestamp: new(1000000),
				TxHash:         new("0xabc123def456"),
			},
			source:        "local",
			blockExplorer: "etherscan.io",
			wantContains: []string{
				"Something happened",
				"block [100](https://etherscan.io/block/100/)",
				"Team test-team",
				"test-bot",
				"TEST-ALERT-1",
				"local",
				"0xabc...456",
			},
		},
		{
			name: "alert without block info",
			alert: &databus.FindingDtoJson{
				Name:        "No Block",
				Description: "desc",
				Severity:    databus.SeverityInfo,
				AlertId:     "TEST-2",
				BotName:     "bot",
				Team:        "team",
				UniqueKey:   "key",
			},
			source:        "src",
			blockExplorer: "etherscan.io",
			wantContains: []string{
				"desc",
				"Team team",
				"bot",
				"TEST-2",
			},
			wantAbsent: []string{
				"block [",
				"Tx hash:",
			},
		},
		{
			name: "alert with empty description",
			alert: &databus.FindingDtoJson{
				Name:        "Empty Desc",
				Description: "",
				Severity:    databus.SeverityCritical,
				AlertId:     "TEST-3",
				BotName:     "bot",
				Team:        "team",
				UniqueKey:   "key",
			},
			source:        "src",
			blockExplorer: "etherscan.io",
			wantContains: []string{
				"Team team",
			},
		},
		{
			name: "alert with tx hash renders shortened hash",
			alert: &databus.FindingDtoJson{
				Name:        "TX Alert",
				Description: "tx alert",
				Severity:    databus.SeverityHigh,
				AlertId:     "TEST-4",
				BotName:     "bot",
				Team:        "team",
				UniqueKey:   "key",
				TxHash:      new("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
			},
			source:        "local",
			blockExplorer: "etherscan.io",
			wantContains: []string{
				"Tx hash: [0x714...bdf](https://etherscan.io/tx/0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf/)",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notifiler.FormatAlert(tt.alert, tt.source, tt.blockExplorer)

			for _, s := range tt.wantContains {
				if !strings.Contains(got, s) {
					t.Errorf("FormatAlert() missing expected substring %q\ngot: %s", s, got)
				}
			}

			for _, s := range tt.wantAbsent {
				if strings.Contains(got, s) {
					t.Errorf("FormatAlert() should not contain %q\ngot: %s", s, got)
				}
			}
		})
	}
}

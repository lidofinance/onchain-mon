package notifiler_test

import (
	"testing"
	"time"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

// Frozen quorum time so the rendered footer can be compared verbatim.
// blockTS below is exactly one hour earlier, so the lag is always 3600s.
var testNow = time.Date(2024, 1, 12, 13, 46, 40, 0, time.UTC)

func TestFormatAlert(t *testing.T) {
	notifiler.Now = func() time.Time { return testNow }
	t.Cleanup(func() { notifiler.Now = time.Now })

	const (
		blockTS = 1705063600 // testNow minus one hour
		txHash  = "0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"
	)

	tests := []struct {
		name  string
		alert *databus.FindingDtoJson
		want  string
	}{
		{
			name: "all fields",
			alert: &databus.FindingDtoJson{
				Description:    "Something happened",
				AlertId:        "TEST-ALERT-1",
				BotName:        "test-bot",
				Team:           "test-team",
				BlockNumber:    new(100),
				BlockTimestamp: new(blockTS),
				TxHash:         new(txHash),
			},
			want: "Something happened\n" +
				"\ntest-team | test-bot | TEST-ALERT-1 | 13:46:40.000 UTC (+3600s) by local\n" +
				"[100](https://etherscan.io/block/100/) | " +
				"[0x714...bdf](https://etherscan.io/tx/" + txHash + "/)",
		},
		{
			name: "no block, no tx",
			alert: &databus.FindingDtoJson{
				Description: "desc",
				AlertId:     "TEST-2",
				BotName:     "bot",
				Team:        "team",
			},
			want: "desc\n\nteam | bot | TEST-2 | 13:46:40.000 UTC by local",
		},
		{
			name: "empty description",
			alert: &databus.FindingDtoJson{
				AlertId: "TEST-3",
				BotName: "bot",
				Team:    "team",
			},
			want: "\nteam | bot | TEST-3 | 13:46:40.000 UTC by local",
		},
		{
			name: "tx hash only, shortened",
			alert: &databus.FindingDtoJson{
				Description: "tx alert",
				AlertId:     "TEST-4",
				BotName:     "bot",
				Team:        "team",
				TxHash:      new(txHash),
			},
			want: "tx alert\n" +
				"\nteam | bot | TEST-4 | 13:46:40.000 UTC by local\n" +
				"[0x714...bdf](https://etherscan.io/tx/" + txHash + "/)",
		},
		{
			// The lag needs only the timestamp, so a block without one still
			// renders its link — just without the "(+Ns)" suffix.
			name: "block without timestamp",
			alert: &databus.FindingDtoJson{
				Description: "desc",
				AlertId:     "TEST-5",
				BotName:     "bot",
				Team:        "team",
				BlockNumber: new(100),
			},
			want: "desc\n" +
				"\nteam | bot | TEST-5 | 13:46:40.000 UTC by local\n" +
				"[100](https://etherscan.io/block/100/)",
		},
		{
			// A timestamp without a block still yields the lag.
			name: "timestamp without block",
			alert: &databus.FindingDtoJson{
				Description:    "desc",
				AlertId:        "TEST-6",
				BotName:        "bot",
				Team:           "team",
				BlockTimestamp: new(blockTS),
			},
			want: "desc\n\nteam | bot | TEST-6 | 13:46:40.000 UTC (+3600s) by local",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notifiler.FormatAlert(tt.alert, "local", "etherscan.io")
			if got != tt.want {
				t.Errorf("FormatAlert()\n got: %q\nwant: %q", got, tt.want)
			}
		})
	}
}

package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/prometheus/client_golang/prometheus"
)

const Name = `[INFO] ℹ️ Lido: BotToken rebased`
const Description = `APR stats
APR: 3.15%
Total shares: 8,123,765.58 (+170686.98) × 1e18
Total pooled ether: 9,489,364.75 (+200180.94) ETH
Time elapsed: 24 hrs 0 sec

Rewards
CL rewards: 727.94 (-2.56) ETH
EL rewards: 181.21 (+2.34) ETH
Total: 909.15 (-0.22) ETH

Validators
Count: 347027 (300 newly appeared)

Withdrawn from vaults
Withdrawal Vault: 310.42 ETH
EL Vault: 181.21 ETH

Requests finalization
Finalized: 180 (20,671.10 ETH)
Pending: 16 (2,906.63 stETH)
Share rate: 1.16810
Used buffer: 20,179.47 ETH

Shares
Burnt: 17,698.36 × 1e18
`

func Test_usecase_SendFinding(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		webhookURL   string
		httpClient   *http.Client
		metricsStore *metrics.Store
	}
	type args struct {
		ctx   context.Context
		alert *databus.FindingDtoJson
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Success",
			fields: fields{
				webhookURL:   cfg.AppConfig.DiscordWebHookURL,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.Background(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ #l2_arbitrum Arbitrum digest",
					Description:    "L1 token rate: 1.1808\nBridge balances:\n\tLDO:\n\t\tL1: 1231218.4603 LDO\n\t\tL2: 1230730.9530 LDO\n\t\n\twstETH:\n\t\tL1: 84477.0663 wstETH\n\t\tL2: 81852.1638 wstETH\n\nWithdrawals:\n\twstETH: 1664.1363 (in 5 transactions)",
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
		{
			name: "Too long message",
			fields: fields{
				webhookURL:   cfg.AppConfig.DiscordWebHookURL,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.Background(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ #l2_arbitrum Arbitrum digest",
					Description:    ParadiseLost,
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := NewDiscord(
				tt.fields.webhookURL,
				tt.fields.httpClient,
				metricsStore,
				`local`,
			)
			if err := u.SendFinding(tt.args.ctx, tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

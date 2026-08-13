//go:build live

// These tests read the repo-root .env / notification.yaml and hit live RPC and
// messaging APIs — running them can post real messages to real channels.
// They are excluded from the default build; run with: go test -tags=live ./...

package notifiler_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

func Test_SendFinding(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	notifcationConfig, err := env.ReadNotificationConfig(cfg.AppConfig.Env, "../../../notification.yaml")
	if err != nil {
		t.Errorf("Read notification config error: %s", err.Error())
		return
	}

	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	transport := &http.Transport{
		MaxIdleConns:          64,
		MaxIdleConnsPerHost:   16,
		MaxConnsPerHost:       12,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	type fields struct {
		botToken     string
		chatID       string
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
			name: "Send_Test_Message_to_telegram",
			fields: fields{
				botToken:     notifcationConfig.TelegramChannels[0].BotToken,
				chatID:       notifcationConfig.TelegramChannels[0].ChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name: `ℹ️ Acme: BotToken rebased`,
					Description: `Withdrawals info:` +
						`requests count:    4302
 withdrawn stTKN:   174541.1742
 finalized stTKN:   174541.1742 4302
 unfinalized stTKN: 0.0000   0
 claimed ether:     142576.2152 853
 unclaimed ether:   31964.9590   3449`,
					Severity:       databus.SeverityLow,
					AlertId:        `ASSET-TOKEN-REBASED`,
					BlockTimestamp: new(1727965236),
					BlockNumber:    new(20884540),
					TxHash:         new("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `FF-tg-unit-test`,
					Team:           `Alpha`,
				},
			},
			wantErr: false,
		},
		{
			name: "Send_Test_Message_to_telegram",
			fields: fields{
				botToken:     notifcationConfig.TelegramChannels[0].BotToken,
				chatID:       notifcationConfig.TelegramChannels[0].ChatID,
				httpClient:   httpClient,
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name: "ℹ️ #l2_beta Beta digest",
					//nolint:lll
					Description:    "L1 token rate: 1.1808\nBridge balances:\n\tGOV:\n\t\tL1: 1231218.4603 GOV\n\t\tL2: 1230730.9530 GOV\n\t\n\twrapTKN:\n\t\tL1: 84477.0663 wrapTKN\n\t\tL2: 81852.1638 wrapTKN\n\nWithdrawals:\n\twrapTKN: 1664.1363 (in 5 transactions)",
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: new(1727965236),
					BlockNumber:    new(20884540),
					TxHash:         new("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Alpha`,
				},
			},
			wantErr: false,
		},
		{
			name: "Send_4096_symbol_to_telegram",
			fields: fields{
				botToken:     notifcationConfig.TelegramChannels[0].BotToken,
				chatID:       notifcationConfig.TelegramChannels[0].ChatID,
				httpClient:   httpClient,
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name:           "ℹ️ Acme: BotToken rebased",
					Description:    ParadiseLost,
					Severity:       databus.SeverityInfo,
					AlertId:        `DIGEST`,
					BlockTimestamp: new(1727965236),
					BlockNumber:    new(20884540),
					TxHash:         new("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Alpha`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := notifiler.NewTelegram(
				tt.fields.botToken,
				tt.fields.chatID,
				tt.fields.httpClient,
				metricsStore,
				`local`,
				`etherscan.io`,
			)
			if err := u.SendFinding(tt.args.ctx, tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

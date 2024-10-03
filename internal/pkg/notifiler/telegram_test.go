package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/generated/databus"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/env"
)

// Helper function to create pointers for strings
func stringPtr(s string) *string {
	return &s
}

// Helper function to create pointers for ints
func intPtr(i int) *int {
	return &i
}

func Test_SendFinfing(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

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
				botToken:     cfg.AppConfig.TelegramBotToken,
				chatID:       cfg.AppConfig.TelegramErrorsChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name: `ℹ️ Lido: Token rebased`,
					Description: `
Withdrawals info:
 requests count:    4302
 withdrawn stETH:   174541.1742
 finalized stETH:   174541.1742 4302
 unfinalized stETH: 0.0000   0
 claimed ether:     142576.2152 853
 unclaimed ether:   31964.9590   3449
`,
					Severity:       databus.SeverityLow,
					AlertId:        `LIDO-TOKEN-REBASED`,
					BlockTimestamp: intPtr(1727965236),
					BlockNumber:    intPtr(20884540),
					TxHash:         stringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `FF-telegram-unit-test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &telegram{
				botToken:   tt.fields.botToken,
				chatID:     tt.fields.chatID,
				httpClient: tt.fields.httpClient,
				metrics:    metricsStore,
				source:     `local`,
			}
			if err := u.SendFinding(tt.args.ctx, tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

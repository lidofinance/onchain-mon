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

const NameCritical = `[CRITICAL] 🚨🚨🚨 ZkSync bridge balance mismatch 🚨🚨🚨`
const DescriptionCritical = `
Total supply of bridged wstETH is greater than balanceOf L1 bridge side!
L2 total supply: 1105.48
L1 balanceOf: 1080.11

ETH: 19811516
ZkSync: 33308621`

func Test_opsGenie_SendMessage(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		opsGenieKey  string
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
				opsGenieKey:  cfg.AppConfig.OpsGenieAPIKey,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx: context.TODO(),
				alert: &databus.FindingDtoJson{
					Name:           NameCritical,
					Description:    DescriptionCritical,
					Severity:       databus.SeverityHigh,
					AlertId:        `TEST-CRITICAL-ID`,
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
			u := &opsGenie{
				opsGenieKey: tt.fields.opsGenieKey,
				httpClient:  tt.fields.httpClient,
				metrics:     metricsStore,
				source:      `local`,
			}
			if err := u.SendFinding(tt.args.ctx, tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

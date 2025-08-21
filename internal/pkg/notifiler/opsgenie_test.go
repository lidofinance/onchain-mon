package notifiler_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/pointers"
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

	notifcationConfig, err := env.ReadNotificationConfig(cfg.AppConfig.Env, "../../../notification.yaml")
	if err != nil {
		t.Errorf("Read notification config error: %s", err.Error())
		return
	}

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
				opsGenieKey:  notifcationConfig.OpsGenieChannels[0].APIKey,
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
					BlockTimestamp: pointers.IntPtr(1727965236),
					BlockNumber:    pointers.IntPtr(20884540),
					TxHash:         pointers.StringPtr("0x714a6c2109c8af671c8a6df594bd9f1f3ba9f11b73a1e54f5f128a3447fa0bdf"),
					BotName:        `Test`,
					Team:           `Protocol`,
				},
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := notifiler.NewOpsgenie(
				tt.fields.opsGenieKey,
				tt.fields.httpClient,
				metricsStore,
				`etherscan.io`,
				`channelId`,
				`opsgenie`,
				`redis-consumer-group-name`,
				`local`,
			)
			if err := u.SendFinding(tt.args.ctx, tt.args.alert, `unit-test-local`); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

const NameCritical = `[CRITICAL] 🚨🚨🚨 ZkSync bridge balance mismatch 🚨🚨🚨`
const DescriptionCritical = `
Total supply of bridged wstETH is greater than balanceOf L1 bridge side!
L2 total supply: 1105.48
L1 balanceOf: 1080.11

ETH: 19811516
ZkSync: 33308621`

func Test_opsGenia_SendMessage(t *testing.T) {
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
		ctx         context.Context
		message     string
		description string
		alias       string
		priority    string
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
				opsGenieKey:  cfg.AppConfig.OpsGeniaAPIKey,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx:         context.TODO(),
				message:     NameCritical,
				description: DescriptionCritical,
				alias:       "ZkSyncBridgeBalanceMismatch",
				priority:    "P2",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &opsGenia{
				opsGenieKey: tt.fields.opsGenieKey,
				httpClient:  tt.fields.httpClient,
				metrics:     metricsStore,
				source:      `local`,
			}
			if err := u.SendMessage(tt.args.ctx, tt.args.message, tt.args.description, tt.args.alias, tt.args.priority); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

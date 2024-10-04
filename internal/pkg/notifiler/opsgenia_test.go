package notifiler

import (
	"testing"
)

const NameCritical = `[CRITICAL] 🚨🚨🚨 ZkSync bridge balance mismatch 🚨🚨🚨`
const DescriptionCritical = `
Total supply of bridged wstETH is greater than balanceOf L1 bridge side!
L2 total supply: 1105.48
L1 balanceOf: 1080.11

ETH: 19811516
ZkSync: 33308621`

func Test_opsGenia_SendMessage(t *testing.T) {
	/*cfg, envErr := env.Read("../../../.env")
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
		alert models.Alert
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
				ctx: context.TODO(),
				alert: models.Alert{
					Name:        NameCritical,
					Description: DescriptionCritical,
					AlertID:     `TEST-OPSGENIA-ALERT-ID`,
					Severity:    models.AlertSeverityHIGH,
				},
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
			if err := u.SendAlert(tt.args.ctx, &tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}*/
}

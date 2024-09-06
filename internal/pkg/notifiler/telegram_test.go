package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/lidofinance/finding-forwarder/generated/forta/models"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

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
		alert models.Alert
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
				alert: models.Alert{
					Name:        `Telegram test alert`,
					Description: `Telegram test alert`,
					Severity:    models.AlertSeverityMEDIUM,
					AlertID:     `telegram-chat-id`,
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
			if err := u.SendAlert(tt.args.ctx, &tt.args.alert); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

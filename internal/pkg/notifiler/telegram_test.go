package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

func Test_SendMessage(t *testing.T) {
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
		ctx     context.Context
		message string
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
				chatID:       cfg.AppConfig.TelegramChatID,
				httpClient:   &http.Client{},
				metricsStore: metricsStore,
			},
			args: args{
				ctx:     context.TODO(),
				message: Name + "\n\n" + Description,
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
			if err := u.SendMessage(tt.args.ctx, tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

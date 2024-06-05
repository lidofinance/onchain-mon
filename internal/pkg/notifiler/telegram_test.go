package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

func Test_SendMessage(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	type fields struct {
		botToken   string
		chatID     string
		httpClient *http.Client
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
				botToken:   cfg.AppConfig.TelegramBotToken,
				chatID:     cfg.AppConfig.TelegramChatID,
				httpClient: &http.Client{},
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
				httpClient: tt.fields.httpClient,
			}
			if err := u.SendMessage(tt.args.ctx, tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

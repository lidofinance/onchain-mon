package usecase

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

const Name = `[CRITICAL] 🚨🚨🚨 ZkSync bridge balance mismatch 🚨🚨🚨`
const Description = `
Total supply of bridged wstETH is greater than balanceOf L1 bridge side!
L2 total supply: 1105.48
L1 balanceOf: 1080.11

ETH: 19811516
ZkSync: 33308621`

func Test_usecase_SendMessage(t *testing.T) {
	cfg, envErr := env.Read("../../../../.env")
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	type fields struct {
		opsGenieKey string
		httpClient  http.Client
	}
	type args struct {
		ctx         context.Context
		message     string
		description string
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
				opsGenieKey: cfg.AppConfig.OpsGeniaAPIKey,
				httpClient:  http.Client{},
			},
			args: args{
				ctx:         context.TODO(),
				message:     Name,
				description: Description,
				priority:    "P4",
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &usecase{
				opsGenieKey: tt.fields.opsGenieKey,
				httpClient:  tt.fields.httpClient,
			}
			if err := u.SendMessage(tt.args.ctx, tt.args.message, tt.args.description, tt.args.priority); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

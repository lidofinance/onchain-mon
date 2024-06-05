package notifiler

import (
	"context"
	"net/http"
	"testing"

	"github.com/lidofinance/finding-forwarder/internal/env"
)

const Name = `[INFO] ℹ️ Lido: Token rebased`
const Description = `APR stats
APR: 3.15%
Total shares: 8,123,765.58 (+170686.98) × 1e18
Total pooled ether: 9,489,364.75 (+200180.94) ETH
Time elapsed: 24 hrs 0 sec

Rewards
CL rewards: 727.94 (-2.56) ETH
EL rewards: 181.21 (+2.34) ETH
Total: 909.15 (-0.22) ETH

Validators
Count: 347027 (300 newly appeared)

Withdrawn from vaults
Withdrawal Vault: 310.42 ETH
EL Vault: 181.21 ETH

Requests finalization
Finalized: 180 (20,671.10 ETH)
Pending: 16 (2,906.63 stETH)
Share rate: 1.16810
Used buffer: 20,179.47 ETH

Shares
Burnt: 17,698.36 × 1e18

ethereum:mainnet
Forta explorer`

func Test_usecase_SendMessage1(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}

	type fields struct {
		webhookURL string
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
			name: "Success",
			fields: fields{
				webhookURL: cfg.AppConfig.DiscordWebHookURL,
				httpClient: &http.Client{},
			},
			args: args{
				ctx:     context.Background(),
				message: Name + "\n\n" + Description,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			u := &discord{
				webhookURL: tt.fields.webhookURL,
				httpClient: tt.fields.httpClient,
			}
			if err := u.SendMessage(tt.args.ctx, tt.args.message); (err != nil) != tt.wantErr {
				t.Errorf("SendMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

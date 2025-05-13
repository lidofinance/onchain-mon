package chain

import (
	"context"
	"errors"
	"net/http"
	"reflect"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
)

func Test_chain_GetLatestBlock(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		jsonRpcUrl string
		httpClient *http.Client
		metrics    *metrics.Store
	}
	type args struct {
		ctx context.Context
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx: context.Background(),
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &chain{
				jsonRpcUrl: tt.fields.jsonRpcUrl,
				httpClient: tt.fields.httpClient,
				metrics:    tt.fields.metrics,
			}
			got, err := c.GetLatestBlock(tt.args.ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetLatestBlock() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got.Result == nil {
				t.Errorf("GetLatestBlock() got nil Result")
				return
			}
		})
	}
}

func Test_chain_GetLatestLogs(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		jsonRpcUrl string
		httpClient *http.Client
		metrics    *metrics.Store
	}
	type args struct {
		ctx       context.Context
		blockHash string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx:       context.Background(),
				blockHash: `0x8d8c264c807984dc36f419759d2d02dde5d7805e18d6da5e6530123101d7b0e6`,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &chain{
				jsonRpcUrl: tt.fields.jsonRpcUrl,
				httpClient: tt.fields.httpClient,
				metrics:    tt.fields.metrics,
			}
			got, err := c.GetBlockReceipts(tt.args.ctx, tt.args.blockHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetBlockReceipts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got.Result == nil || len(*got.Result) == 0 {
				t.Errorf("GetBlockReceipts() got nil Result")
				return
			}

			blocKReceipts := *got.Result

			if blocKReceipts[0].BlockHash != tt.args.blockHash {
				t.Errorf("GetBlockReceipts() got %s, want %s", blocKReceipts[0].BlockHash, tt.args.blockHash)
				return
			}
		})
	}
}

func Test_chain_FetchBlockByNumber(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		jsonRpcUrl string
		httpClient *http.Client
		metrics    *metrics.Store
	}
	type args struct {
		ctx         context.Context
		blockNumber int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int64
		wantErr error
	}{
		{
			name: "Nil response",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx:         context.Background(),
				blockNumber: 92466198,
			},
			want:    0,
			wantErr: EmptyResponseErr,
		},
		{
			name: "success",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx:         context.Background(),
				blockNumber: 22466927,
			},
			want:    22466927,
			wantErr: EmptyResponseErr,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &chain{
				jsonRpcUrl: tt.fields.jsonRpcUrl,
				httpClient: tt.fields.httpClient,
				metrics:    tt.fields.metrics,
			}
			got, err := c.FetchBlockByNumber(tt.args.ctx, tt.args.blockNumber)
			if err != nil && tt.wantErr != nil {
				if errors.Is(err, tt.wantErr) {
					return
				}

				t.Errorf("Wrong expected error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if !reflect.DeepEqual(got.Result.GetNumber(), tt.want) {
				t.Errorf("FetchBlockByNumber() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_chain_FetchBlocksInRange(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		jsonRpcUrl string
		httpClient *http.Client
		metrics    *metrics.Store
	}
	type args struct {
		ctx  context.Context
		from int64
		to   int64
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx:  context.Background(),
				from: 22475140,
				to:   22475144,
			},
			want:    5,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &chain{
				jsonRpcUrl: tt.fields.jsonRpcUrl,
				httpClient: tt.fields.httpClient,
				metrics:    tt.fields.metrics,
			}
			got, err := c.FetchBlocksInRange(tt.args.ctx, tt.args.from, tt.args.to)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchBlocksInRange() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(len(*got.Result), tt.want) {
				t.Errorf("FetchBlocksInRange() got = %v, want %v", len(*got.Result), tt.want)
			}
		})
	}
}

func Test_chain_FetchReceipts(t *testing.T) {
	cfg, envErr := env.Read("../../../.env")
	if envErr != nil {
		t.Errorf("Read env error: %s", envErr.Error())
		return
	}
	promRegistry := prometheus.NewRegistry()
	metricsStore := metrics.New(promRegistry, cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)

	type fields struct {
		jsonRpcUrl string
		httpClient *http.Client
		metrics    *metrics.Store
	}
	type args struct {
		ctx         context.Context
		blockHashes []string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "success",
			fields: fields{
				jsonRpcUrl: cfg.AppConfig.JsonRpcURL,
				httpClient: &http.Client{},
				metrics:    metricsStore,
			},
			args: args{
				ctx: context.Background(),
				blockHashes: []string{
					"0x80e24fe788f4e2356f448765c13e217c99aa060e47e4cc822bde3c8c9c0e79e2",
					"0xa3eb409d960635e18ba03baabeaa5ab8fd8b1e146fe1360978df4b6744d0bec4",
					"0x684d08620238daf4a52f45b6610fcb06d4af11a32ad63e4b22c61ae3200922ac",
					"0xa96d4ed993006d2d38f61c2b8cd81e1e4f3e595a5760a563c20a6ca386a462ef",
					"0x8baae82c42e386c5a62e886b64fe3d443da5f92d5dc737b290dc9bb0af96e8db",
				},
			},
			want:    1114,
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &chain{
				jsonRpcUrl: tt.fields.jsonRpcUrl,
				httpClient: tt.fields.httpClient,
				metrics:    tt.fields.metrics,
			}
			got, err := c.FetchReceipts(tt.args.ctx, tt.args.blockHashes)
			if (err != nil) != tt.wantErr {
				t.Errorf("FetchReceipts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(*got.Result) != tt.want {
				t.Errorf("FetchReceipts() got = %v, want %v", len(*got.Result), tt.want)
			}
		})
	}
}

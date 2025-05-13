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

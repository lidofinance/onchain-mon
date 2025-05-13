package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/avast/retry-go/v4"
	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain/entity"
)

type chain struct {
	jsonRpcUrl string
	httpClient *http.Client
	metrics    *metrics.Store
}

var EmptyResponseErr = errors.New("empty response")

func NewChain(jsonRpcUrl string, httpClient *http.Client, metricsStore *metrics.Store) *chain {
	return &chain{
		jsonRpcUrl: jsonRpcUrl,
		httpClient: httpClient,
		metrics:    metricsStore,
	}
}

func (c *chain) GetLatestBlock(ctx context.Context) (*entity.RpcResponse[entity.EthBlock], error) {
	return doRpcRequest[entity.EthBlock](ctx, "eth_getBlockByNumber", []any{"latest", false}, c.httpClient, c.metrics, c.jsonRpcUrl)
}

func (c *chain) GetBlockReceipts(ctx context.Context, blockHash string) (*entity.RpcResponse[[]entity.BlockReceipt], error) {
	return doRpcRequest[[]entity.BlockReceipt](ctx, "eth_getBlockReceipts", []any{blockHash}, c.httpClient, c.metrics, c.jsonRpcUrl)
}

func (c *chain) FetchBlockByNumber(ctx context.Context, blockNumber int64) (*entity.RpcResponse[entity.EthBlock], error) {
	hexValue := fmt.Sprintf("0x%x", blockNumber)
	return doRpcRequest[entity.EthBlock](ctx, "eth_getBlockByNumber", []any{hexValue, false}, c.httpClient, c.metrics, c.jsonRpcUrl)
}

func doRpcRequest[T any](
	ctx context.Context, method string, params []any,
	httpClient *http.Client, m *metrics.Store, jsonRpcUrl string,
) (*entity.RpcResponse[T], error) {
	return retry.DoWithData(
		func() (*entity.RpcResponse[T], error) {
			rpcRequest := entity.RpcRequest{
				JsonRpc: "2.0",
				Method:  method,
				Params:  params,
				ID:      uuid.New().String(),
			}

			payload, marshaErr := json.Marshal(rpcRequest)
			if marshaErr != nil {
				return nil, marshaErr
			}

			req, err := http.NewRequestWithContext(ctx, "POST", jsonRpcUrl, bytes.NewBuffer(payload))
			if err != nil {
				return nil, fmt.Errorf("could not create request: %w", err)
			}

			req.Header.Set("Content-Type", "application/json")

			start := time.Now()
			resp, err := httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("could not send request: %w", err)
			}
			defer func() {
				resp.Body.Close()
				duration := time.Since(start).Seconds()
				m.SummaryHandlers.With(prometheus.Labels{metrics.Channel: method}).Observe(duration)
			}()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("could not read response body: %w", err)
			}

			var p entity.RpcResponse[T]
			if err := json.Unmarshal(body, &p); err != nil {
				return nil, fmt.Errorf("could not unmarshal response: %w", err)
			}

			if p.Error != nil {
				return nil, fmt.Errorf("RPC code(%d) error: %s", p.Error.Code, p.Error.Message)
			}

			if p.Result == nil {
				return nil, fmt.Errorf("%s rpcResponse.Result is nil. payload %s: %w", method, string(payload), EmptyResponseErr)
			}

			return &p, nil
		},
		retry.Attempts(5),
		retry.Delay(750*time.Millisecond),
		retry.Context(ctx),
		retry.RetryIf(func(err error) bool {
			if errors.Is(err, EmptyResponseErr) {
				return false
			}
			return true
		}),
	)
}

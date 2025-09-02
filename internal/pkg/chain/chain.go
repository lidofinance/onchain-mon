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

var ErrEmptyResponse = errors.New("empty response")

const MaxAttempts = 6
const RetryDelay = 75 * time.Millisecond
const MaxDelay = 5 * time.Second

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
				return nil, fmt.Errorf("%s rpcResponse.Result is nil. payload %s: %w", method, string(payload), ErrEmptyResponse)
			}

			return &p, nil
		},
		retry.Attempts(MaxAttempts),
		retry.Delay(RetryDelay),
		retry.MaxDelay(MaxDelay),
		retry.DelayType(retry.CombineDelay(
			retry.BackOffDelay,
			retry.RandomDelay,
		)),
		retry.RetryIf(func(err error) bool {
			return !errors.Is(err, ErrEmptyResponse)
		}),
	)
}

func (c *chain) FetchReceipts(ctx context.Context, blockHashes []string) (*entity.RpcResponse[[]entity.BlockReceipt], error) {
	if len(blockHashes) == 0 {
		return &entity.RpcResponse[[]entity.BlockReceipt]{Result: &[]entity.BlockReceipt{}}, nil
	}

	return retry.DoWithData(
		func() (*entity.RpcResponse[[]entity.BlockReceipt], error) {
			requests := make([]entity.RpcRequest, 0, len(blockHashes))
			for _, hash := range blockHashes {
				requests = append(requests, entity.RpcRequest{
					JsonRpc: "2.0",
					Method:  "eth_getBlockReceipts",
					Params:  []any{hash},
					ID:      hash,
				})
			}

			payload, err := json.Marshal(requests)
			if err != nil {
				return nil, fmt.Errorf("marshal batch: %w", err)
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.jsonRpcUrl, bytes.NewBuffer(payload))
			if err != nil {
				return nil, fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("send request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read body: %w", err)
			}

			var rawResponses []entity.RpcResponse[[]entity.BlockReceipt]
			if err := json.Unmarshal(body, &rawResponses); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w", err)
			}

			var combined []entity.BlockReceipt
			for _, r := range rawResponses {
				if r.Error != nil {
					return nil, fmt.Errorf("rpc error: %s", r.Error.Message)
				}
				if r.Result != nil {
					combined = append(combined, *r.Result...)
				}
			}

			return &entity.RpcResponse[[]entity.BlockReceipt]{
				JsonRpc: "2.0",
				ID:      "batch",
				Result:  &combined,
			}, nil
		},
		retry.Attempts(MaxAttempts),
		retry.Delay(RetryDelay),
		retry.MaxDelay(MaxDelay),
		retry.DelayType(retry.CombineDelay(
			retry.BackOffDelay,
			retry.RandomDelay,
		)),
		retry.Context(ctx),
	)
}

func (c *chain) FetchBlocksInRange(ctx context.Context, from, to int64) (*entity.RpcResponse[[]entity.EthBlock], error) {
	if from > to {
		return &entity.RpcResponse[[]entity.EthBlock]{Result: &[]entity.EthBlock{}}, nil
	}

	return retry.DoWithData(
		func() (*entity.RpcResponse[[]entity.EthBlock], error) {
			requests := make([]entity.RpcRequest, 0, to-from+1)
			for i := from; i <= to; i++ {
				hexNum := fmt.Sprintf("0x%x", i)
				requests = append(requests, entity.RpcRequest{
					JsonRpc: "2.0",
					Method:  "eth_getBlockByNumber",
					Params:  []any{hexNum, false},
					ID:      hexNum,
				})
			}

			payload, err := json.Marshal(requests)
			if err != nil {
				return nil, fmt.Errorf("marshal batch: %w", err)
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.jsonRpcUrl, bytes.NewBuffer(payload))
			if err != nil {
				return nil, fmt.Errorf("create request: %w", err)
			}
			req.Header.Set("Content-Type", "application/json")

			resp, err := c.httpClient.Do(req)
			if err != nil {
				return nil, fmt.Errorf("send request: %w", err)
			}
			defer resp.Body.Close()

			body, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, fmt.Errorf("read body: %w", err)
			}

			var rawResponses []entity.RpcResponse[entity.EthBlock]
			if err := json.Unmarshal(body, &rawResponses); err != nil {
				return nil, fmt.Errorf("unmarshal response: %w", err)
			}

			blocks := make([]entity.EthBlock, 0, len(rawResponses))
			for _, r := range rawResponses {
				if r.Error != nil {
					return nil, fmt.Errorf("rpc error: %s", r.Error.Message)
				}
				if r.Result != nil {
					blocks = append(blocks, *r.Result)
				}
			}

			return &entity.RpcResponse[[]entity.EthBlock]{
				JsonRpc: "2.0",
				ID:      "block-batch",
				Result:  &blocks,
			}, nil
		},
		retry.Attempts(MaxAttempts),
		retry.Delay(RetryDelay),
		retry.MaxDelay(MaxDelay),
		retry.DelayType(retry.CombineDelay(
			retry.BackOffDelay,
			retry.RandomDelay,
		)),
		retry.Context(ctx),
	)
}

package feeder

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/klauspost/compress/zstd"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain/entity"
)

type ChainSrv interface {
	GetLatestBlock(ctx context.Context) (*entity.RpcResponse[entity.EthBlock], error)
	GetBlockReceipts(ctx context.Context, blockHash string) (*entity.RpcResponse[[]entity.BlockReceipt], error)
	FetchBlockByNumber(ctx context.Context, blockNumber int64) (*entity.RpcResponse[entity.EthBlock], error)
}

type Feeder struct {
	log          *slog.Logger
	chainSrv     ChainSrv
	js           jetstream.JetStream
	metricsStore *metrics.Store
	topic        string
}

func New(log *slog.Logger, chainSrv ChainSrv, js jetstream.JetStream, metricsStore *metrics.Store, topic string) *Feeder {
	return &Feeder{
		log:          log,
		chainSrv:     chainSrv,
		js:           js,
		metricsStore: metricsStore,
		topic:        topic,
	}
}

const Per6Sec = 6 * time.Second
const maxPayloadSize4mb = 4 * 1024 * 1024

func (w *Feeder) Run(ctx context.Context, g *errgroup.Group) {
	prevHashBlock := ""

	g.Go(func() error {
		ticker := time.NewTicker(Per6Sec)
		defer ticker.Stop()

		prevBlockNumber := int64(-1)

		var (
			block          *entity.RpcResponse[entity.EthBlock]
			fetchStartTime time.Time
			err            error
		)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				if prevBlockNumber == -1 {
					block, err = w.chainSrv.GetLatestBlock(ctx)
					if err != nil {
						w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
						w.log.Error(fmt.Sprintf("GetLatestBlock error: %v", err))
						continue
					}
				} else {
					nextBlockNumber := prevBlockNumber + 1
					block, err = w.chainSrv.FetchBlockByNumber(ctx, nextBlockNumber)
					if err != nil {
						if fetchStartTime.IsZero() {
							fetchStartTime = time.Now()
						}

						if !errors.Is(err, chain.EmptyResponseErr) {
							w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
							w.log.Error(fmt.Sprintf("FetchBlockByNumber error: %v", err))
						} else {
							ticker.Reset(2 * time.Second)
							w.log.Debug(fmt.Sprintf("Block %d not yet available", nextBlockNumber))
						}

						if time.Since(fetchStartTime) > 18*time.Second {
							if prevBlockNumber != -1 {
								w.log.Warn("Too long without next block, resetting to latest")
								prevBlockNumber = -1
								fetchStartTime = time.Time{}
								w.metricsStore.BlockResets.Inc()
							}
						}

						continue
					}

					fetchStartTime = time.Time{}
				}

				if block.Result.Hash == prevHashBlock {
					continue
				}

				blockReceipts, err := w.chainSrv.GetBlockReceipts(ctx, block.Result.Hash)
				if err != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("GetBlockReceipts error: %v", err))
					continue
				}

				receipts := make([]databus.BlockDtoJsonReceiptsElem, 0, len(*blockReceipts.Result))
				for i := range *blockReceipts.Result {
					receipt := (*blockReceipts.Result)[i]
					logs := make([]databus.BlockDtoJsonReceiptsElemLogsElem, 0, len(receipt.Logs))
					for j := range receipt.Logs {
						receiptLog := receipt.Logs[j]
						blockNumber, _ := strconv.ParseInt(receiptLog.BlockNumber, 0, 64)
						logIndex, _ := strconv.ParseInt(receiptLog.LogIndex, 0, 64)
						trxInd, _ := strconv.ParseInt(receiptLog.TransactionIndex, 0, 64)

						logs = append(logs, databus.BlockDtoJsonReceiptsElemLogsElem{
							Address:          receiptLog.Address,
							BlockHash:        receiptLog.BlockHash,
							BlockNumber:      int(blockNumber),
							Data:             receiptLog.Data,
							LogIndex:         int(logIndex),
							Removed:          receiptLog.Removed,
							Topics:           receiptLog.Topics,
							TransactionHash:  receiptLog.TransactionHash,
							TransactionIndex: int(trxInd),
						})
					}

					receipts = append(receipts, databus.BlockDtoJsonReceiptsElem{
						Logs:            logs,
						To:              &receipt.To,
						From:            receipt.From,
						TransactionHash: receipt.TransactionHash,
					})
				}

				blockDto := databus.BlockDtoJson{
					Hash:       block.Result.Hash,
					Number:     int(block.Result.GetNumber()),
					ParentHash: block.Result.ParentHash,
					Receipts:   receipts,
					Timestamp:  int(block.Result.GetTimestamp()),
				}

				payload, marshalErr := json.Marshal(blockDto)
				if marshalErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf(`Could not marshal blockDto %s`, marshalErr))
					continue
				}

				cPayload, compressErr := compress(payload)
				if compressErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf(`Could not compress blockDto by zstd: %s`, compressErr))
					continue
				}

				payloadSize := slog.String("payloadSize", fmt.Sprintf(`%.6f mb`, float64(len(payload))/(1024*1024)))
				cPayloadSize := slog.String("cPayloadSize", fmt.Sprintf(`%.6f mb`, float64(cPayload.Len())/(1024*1024)))

				if cPayload.Len() > maxPayloadSize4mb {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error("Payload too large to publish", slog.Int("size", cPayload.Len()))
					continue
				}

				prevHashBlock = block.Result.Hash
				if _, publishErr := w.js.PublishAsync(w.topic, cPayload.Bytes(),
					jetstream.WithMsgID(blockDto.Hash),
					//nolint
					jetstream.WithRetryAttempts(5),
					//nolint
					jetstream.WithRetryWait(250*time.Millisecond),
				); publishErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("could not publish block %d to JetStream: error: %v, ", blockDto.Number, publishErr),
						payloadSize,
					)
					continue
				}

				w.log.Info(fmt.Sprintf(`%d, %s`, blockDto.Number, blockDto.Hash), payloadSize, cPayloadSize)
				w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusOk}).Inc()
				prevBlockNumber = block.Result.GetNumber()

				// estimate next block time
				expectedNextBlockTime := time.Unix(block.Result.GetTimestamp(), 0).Add(12 * time.Second)
				delay := time.Until(expectedNextBlockTime.Add(500 * time.Millisecond))

				if delay < time.Second {
					delay = time.Second
				}

				ticker.Reset(delay)
				w.log.Debug("Adjusted ticker delay", slog.Duration("delay", delay))
			}
		}
	})

	g.Go(func() error {
		<-ctx.Done()
		return nil
	})
}

func compress(payload []byte) (*bytes.Buffer, error) {
	cPayload := &bytes.Buffer{}
	zstdWriter, _ := zstd.NewWriter(cPayload)
	defer zstdWriter.Close()

	if _, zstdErr := zstdWriter.Write(payload); zstdErr != nil {
		return nil, zstdErr
	}

	return cPayload, nil
}

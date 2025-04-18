package feeder

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain/entity"
)

type ChainSrv interface {
	GetLatestBlock(ctx context.Context) (*entity.RpcResponse[entity.EthBlock], error)
	GetBlockReceipts(ctx context.Context, blockHash string) (*entity.RpcResponse[[]entity.BlockReceipt], error)
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

func (w *Feeder) Run(ctx context.Context, g *errgroup.Group) {
	prevHashBlock := ""

	g.Go(func() error {
		ticker := time.NewTicker(Per6Sec)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				block, err := w.chainSrv.GetLatestBlock(ctx)
				if err != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("GetLatestBlock error: %v", err))
					continue
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

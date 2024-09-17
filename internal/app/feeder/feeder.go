package feeder

import (
	"context"
	"encoding/json"
	"fmt"
	databus "github.com/lidofinance/finding-forwarder/generated/databaus"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	"github.com/lidofinance/finding-forwarder/internal/pkg/chain/entity"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"log/slog"
	"strconv"
	"time"
)

type ChainSrv interface {
	GetLatestBlock(ctx context.Context) (*entity.RpcResponse[entity.EthBlock], error)
	GetBlockReceipts(ctx context.Context, blockHash string) (*entity.RpcResponse[[]entity.BlockReceipt], error)
}

type Feeder struct {
	log      *slog.Logger
	chainSrv ChainSrv
	js       jetstream.JetStream
	metrics  *metrics.Store
	topic    string
}

func New(log *slog.Logger, chainSrv ChainSrv, js jetstream.JetStream, metrics *metrics.Store, topic string) *Feeder {
	return &Feeder{
		log:      log,
		chainSrv: chainSrv,
		js:       js,
		metrics:  metrics,
		topic:    topic,
	}
}

func (w *Feeder) Run(ctx context.Context, g *errgroup.Group) {
	prevHashBlock := ""

	g.Go(func() error {
		ticker := time.NewTicker(6 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-ticker.C:
				block, err := w.chainSrv.GetLatestBlock(ctx)
				if err != nil {
					w.log.Error("GetLatestBlock error:", err.Error())
					continue
				}

				if block.Result.Hash == prevHashBlock {
					continue
				}

				blockReceipts, err := w.chainSrv.GetBlockReceipts(ctx, block.Result.Hash)
				if err != nil {
					w.log.Error("GetBlockReceipts error:", err.Error())
					continue
				}

				receipts := make([]databus.BlockDtoJsonReceiptsElem, 0, len(*blockReceipts.Result))
				for _, receipt := range *blockReceipts.Result {
					logs := make([]databus.BlockDtoJsonReceiptsElemLogsElem, 0, len(receipt.Logs))
					for _, receiptLog := range receipt.Logs {
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
						Logs: logs,
						To:   &receipt.To,
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
					w.log.Error(fmt.Sprintf(`Could not marshal blockDto %s`, marshalErr))
					continue
				}

				prevHashBlock = block.Result.Hash
				if _, publishErr := w.js.PublishAsync(w.topic, payload,
					jetstream.WithMsgID(blockDto.Hash),
					jetstream.WithRetryAttempts(5),
					jetstream.WithRetryWait(250*time.Millisecond),
				); publishErr != nil {
					w.metrics.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("could not publish block %d to JetStream: error: %v", blockDto.Number, publishErr))
					continue
				}

				w.log.Info(fmt.Sprintf(`%d, %s`, blockDto.Number, blockDto.Hash))
				w.metrics.PublishedAlerts.With(prometheus.Labels{metrics.Status: metrics.StatusOk}).Inc()
			}
		}
	})

	g.Go(func() error {
		<-ctx.Done()
		return nil
	})
}

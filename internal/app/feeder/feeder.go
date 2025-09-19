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
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain"
	"github.com/lidofinance/onchain-mon/internal/pkg/chain/entity"
)

type ChainSrv interface {
	GetLatestBlock(ctx context.Context) (*entity.RpcResponse[entity.EthBlock], error)
	GetBlockNumber(ctx context.Context) (*entity.RpcResponse[string], error)
	GetBlockReceipts(ctx context.Context, blockHash string) (*entity.RpcResponse[[]entity.BlockReceipt], error)
	FetchReceipts(ctx context.Context, blockHashes []string) (*entity.RpcResponse[[]entity.BlockReceipt], error)
	FetchBlockByNumber(ctx context.Context, blockNumber int64) (*entity.RpcResponse[entity.EthBlock], error)
	FetchBlocksInRange(ctx context.Context, blockNumber int64, latestNumber int64) (*entity.RpcResponse[[]entity.EthBlock], error)
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
const JetStreamRetryWrite = 250 * time.Millisecond
const JetStreamAttemptsWrite = 5
const EtaNextBlock = 12 * time.Second
const DelayNextBlock = 500 * time.Millisecond

func (w *Feeder) Run(ctx context.Context, g *errgroup.Group) {
	g.Go(func() error {
		timer := time.NewTimer(Per6Sec)
		defer timer.Stop()

		prevBlockNumber := int64(-1)

		var (
			fetchStartTime time.Time
		)

		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-timer.C:
				// First, check the current block number
				blockNumberResp, err := w.chainSrv.GetBlockNumber(ctx)
				if err != nil {
					w.log.Error(fmt.Sprintf("GetBlockNumber error: %v", err))
					w.resetTimer(timer)
					continue
				}

				if blockNumberResp == nil || blockNumberResp.Result == nil {
					w.log.Error("Received nil block number response")
					w.resetTimer(timer)
					continue
				}

				// Convert hex block number to int64
				currentBlockNumber, err := strconv.ParseInt(*blockNumberResp.Result, 0, 64)
				if err != nil {
					w.log.Error(fmt.Sprintf("Failed to parse block number %s: %v", *blockNumberResp.Result, err))
					w.resetTimer(timer)
					continue
				}

				// Initialize prevBlockNumber if this is the first run
				if prevBlockNumber == -1 {
					prevBlockNumber = currentBlockNumber - 1
				}

				// Check if there's a new block
				if currentBlockNumber <= prevBlockNumber {
					w.log.Debug(fmt.Sprintf("No new block yet. Current: %d, Previous: %d", currentBlockNumber, prevBlockNumber))
					w.resetTimer(timer)
					continue
				}

				// There's at least one new block, fetch the next expected block
				nextBlockNumber := prevBlockNumber + 1

				// Check if we've been waiting too long for a specific block
				if nextBlockNumber < currentBlockNumber && !fetchStartTime.IsZero() && time.Since(fetchStartTime) > 2*time.Minute {
					w.metricsStore.BlockResets.Inc()
					w.log.Warn(fmt.Sprintf("Too long without next block %d, current is %d, attempting to recover missed blocks", nextBlockNumber, currentBlockNumber))
					latestRecovered, recoverErr := w.recoverMissedBlocks(ctx, prevBlockNumber)
					if recoverErr != nil {
						w.log.Error(fmt.Sprintf("Failed to recover missed blocks %s", recoverErr))
						prevBlockNumber = currentBlockNumber - 1
						w.resetTimer(timer)
					} else {
						prevBlockNumber = latestRecovered.GetNumber()
						delay := w.updateTickerAfterBlock(timer, latestRecovered)
						delay = max(delay, 30*time.Second)
						w.log.Info(fmt.Sprintf("Latest recovered block: %d, delay: %s", (*latestRecovered).GetNumber(), slog.Duration("delay", delay)))
					}
					fetchStartTime = time.Time{}
					continue
				}

				// Fetch the next block
				block, err := w.chainSrv.FetchBlockByNumber(ctx, nextBlockNumber)
				if err != nil {
					if fetchStartTime.IsZero() {
						fetchStartTime = time.Now()
					}

					if errors.Is(err, chain.ErrEmptyResponse) {
						w.log.Info(fmt.Sprintf("Block %d is not available yet", nextBlockNumber))
						w.resetTimer(timer)
						continue
					}

					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("FetchBlockByNumber error: %v", err))
					w.resetTimer(timer)
					continue
				}

				fetchStartTime = time.Time{}

				if block == nil || block.Result == nil {
					w.log.Error("Received nil block or result")
					w.resetTimer(timer)
					continue
				}

				// Fetch block receipts
				blockReceipts, getReceiptsErr := w.chainSrv.GetBlockReceipts(ctx, block.Result.Hash)
				if getReceiptsErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("GetBlockReceipts error: %v", getReceiptsErr))
					w.resetTimer(timer)
					continue
				}

				// Build and publish the block
				blockDto := buildBlockDto(block.Result, *blockReceipts.Result)
				if publishErr := w.publishBlock(blockDto); publishErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf(`Could not publish block blockDto: %s`, publishErr))
					w.resetTimer(timer)
					continue
				}

				w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusOk}).Inc()
				prevBlockNumber = block.Result.GetNumber()
				delay := w.updateTickerAfterBlock(timer, block.Result)

				w.log.Info(fmt.Sprintf("%d is published. next block delay %s", blockDto.Number, slog.Duration("delay", delay)))
			}
		}
	})
}

func (w *Feeder) recoverMissedBlocks(ctx context.Context, fromBlock int64) (*entity.EthBlock, error) {
	latestBlock, err := w.chainSrv.GetLatestBlock(ctx)
	if err != nil {
		return nil, fmt.Errorf("get latest block: %w", err)
	}

	latestNumber := latestBlock.Result.GetNumber()
	if latestNumber <= fromBlock {
		return nil, fmt.Errorf("latest block number %d is less than fromBlock %d", latestNumber, fromBlock)
	}

	blocksResp, err := w.chainSrv.FetchBlocksInRange(ctx, fromBlock+1, latestNumber)
	if err != nil {
		return nil, fmt.Errorf("fetch blocksResp in range: %w", err)
	}

	blockHashes := make([]string, 0, len(*blocksResp.Result))
	for i := range *blocksResp.Result {
		blockHashes = append(blockHashes, (*blocksResp.Result)[i].Hash)
	}

	receiptsResp, err := w.chainSrv.FetchReceipts(ctx, blockHashes)
	if err != nil {
		return nil, fmt.Errorf("could not fetch receipts: %w", err)
	}

	receiptsByBlock := make(map[string][]entity.BlockReceipt, len(*receiptsResp.Result))
	for i := range *receiptsResp.Result {
		receipt := (*receiptsResp.Result)[i]
		receiptsByBlock[receipt.BlockHash] = append(receiptsByBlock[receipt.BlockHash], receipt)
	}

	var latestPubBlock *entity.EthBlock = nil
	for i := range *blocksResp.Result {
		block := (*blocksResp.Result)[i]
		hash := block.Hash
		receipts := receiptsByBlock[hash]

		dto := buildBlockDto(&block, receipts)
		if publishErr := w.publishBlock(dto); publishErr != nil {
			return nil, fmt.Errorf("could not publish block: %w", publishErr)
		}
		latestPubBlock = &block

		w.log.Info(fmt.Sprintf("Recovered block %d", dto.Number))
	}

	return latestPubBlock, nil
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

func buildBlockDto(block *entity.EthBlock, blockReceipts []entity.BlockReceipt) databus.BlockDtoJson {
	receipts := make([]databus.BlockDtoJsonReceiptsElem, 0, len(blockReceipts))
	for i := range blockReceipts {
		receipt := blockReceipts[i]
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

	return databus.BlockDtoJson{
		Hash:       block.Hash,
		Number:     int(block.GetNumber()),
		ParentHash: block.ParentHash,
		Receipts:   receipts,
		Timestamp:  int(block.GetTimestamp()),
	}
}

func (w *Feeder) publishBlock(blockDto databus.BlockDtoJson) error {
	payload, marshalErr := json.Marshal(blockDto)
	if marshalErr != nil {
		return fmt.Errorf("could not marshal blockDto: %w", marshalErr)
	}

	cPayload, compressErr := compress(payload)
	if compressErr != nil {
		return fmt.Errorf("could not compress blockDto by zstd: %w", compressErr)
	}

	if _, publishErr := w.js.PublishAsync(w.topic, cPayload.Bytes(),
		jetstream.WithMsgID(blockDto.Hash),
		jetstream.WithRetryAttempts(JetStreamAttemptsWrite),
		jetstream.WithRetryWait(JetStreamRetryWrite),
	); publishErr != nil {
		cPayloadSize := slog.String("cPayloadSize", fmt.Sprintf(`%.6f mb`, float64(cPayload.Len())/(1024*1024)))
		return fmt.Errorf("could not publish block %d(cPayload: %s) to JetStream: %w ", blockDto.Number, cPayloadSize, publishErr)
	}

	return nil
}

func (w *Feeder) updateTickerAfterBlock(timer *time.Timer, block *entity.EthBlock) time.Duration {
	expectedNextBlockTime := time.Unix(block.GetTimestamp(), 0).Add(EtaNextBlock)
	delay := time.Until(expectedNextBlockTime.Add(DelayNextBlock))

	// Trim delay to be at least 1 second and at most 30 seconds
	// Upper bound is needed for the case when the block is far in the future (forked env)
	delay = max(1*time.Second, min(delay, 30*time.Second))

	timer.Reset(delay)
	return delay
}

func (w *Feeder) resetTimer(timer *time.Timer) {
	// Reset timer to 2 seconds to avoid spamming the network
	timer.Reset(2 * time.Second)
}

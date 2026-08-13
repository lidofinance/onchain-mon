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

// How many blocks one recovery batch asks for. Keep it well under the RPC
// client timeout: each block also drags its receipts along.
const RecoverChunkSize = 50

func (w *Feeder) Run(ctx context.Context, g *errgroup.Group) {
	g.Go(func() error {
		timer := time.NewTimer(Per6Sec)
		defer timer.Stop()

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
			case <-timer.C:
				if prevBlockNumber == -1 {
					block, err = w.chainSrv.GetLatestBlock(ctx)
					if err != nil {
						w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
						w.log.Error(fmt.Sprintf("GetLatestBlock error: %v", err))
						w.resetTimer(timer)
						continue
					}
				} else {
					nextBlockNumber := prevBlockNumber + 1
					block, err = w.chainSrv.FetchBlockByNumber(ctx, nextBlockNumber)
					if err != nil {
						if fetchStartTime.IsZero() {
							fetchStartTime = time.Now()
						}

						if errors.Is(err, chain.ErrEmptyResponse) {
							w.log.Info(fmt.Sprintf("Block %d is not available", nextBlockNumber))
							w.resetTimer(timer)
							continue
						}

						w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
						w.log.Error(fmt.Sprintf("FetchBlockByNumber error: %v", err))

						if time.Since(fetchStartTime) > 2*time.Minute {
							w.metricsStore.BlockResets.Inc()
							w.log.Warn("Too long without next block, attempting to recover missed blocks")
							latestRecovered, recoverErr := w.recoverMissedBlocks(ctx, prevBlockNumber)
							if recoverErr != nil {
								w.log.Error(fmt.Sprintf("Failed to recover missed blocks %s", recoverErr))
								prevBlockNumber = -1
								w.resetTimer(timer)
							} else {
								prevBlockNumber = latestRecovered.GetNumber()
								delay := w.updateTickerAfterBlock(timer, latestRecovered)
								w.log.Info(fmt.Sprintf("Latest recovered block: %d, delay: %s", (*latestRecovered).GetNumber(), slog.Duration("delay", delay)))
							}

							// Positive branch - we recovered all skipped blocks
							fetchStartTime = time.Time{}
							continue
						}

						w.resetTimer(timer)
						continue
					}

					fetchStartTime = time.Time{}
				}

				if block == nil || block.Result == nil {
					w.log.Error("Received nil block or result")
					w.resetTimer(timer)
					continue
				}

				if block.Result.GetNumber() == prevBlockNumber {
					w.resetTimer(timer)
					w.log.Warn(fmt.Sprintf(`got the same block %d as before, skipping`, block.Result.GetNumber()))
					continue
				}

				blockReceipts, getReceiptsErr := w.chainSrv.GetBlockReceipts(ctx, block.Result.Hash)
				if getReceiptsErr != nil {
					w.metricsStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					w.log.Error(fmt.Sprintf("GetBlockReceipts error: %v", getReceiptsErr))
					w.resetTimer(timer)
					continue
				}

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

	if latestBlock.Result == nil {
		return nil, fmt.Errorf("get latest block: %w", chain.ErrEmptyResponse)
	}

	latestNumber := latestBlock.Result.GetNumber()
	if latestNumber <= fromBlock {
		return nil, fmt.Errorf("latest block number %d is less than fromBlock %d", latestNumber, fromBlock)
	}

	// Walk the gap in chunks: a long RPC outage can leave hundreds of blocks
	// behind, and asking for all of them (plus their receipts) in one batch
	// runs into the client timeout and loses the whole recovery.
	var latestPubBlock *entity.EthBlock
	for from := fromBlock + 1; from <= latestNumber; from += RecoverChunkSize {
		to := min(from+RecoverChunkSize-1, latestNumber)

		lastInChunk, chunkErr := w.recoverBlockRange(ctx, from, to)
		if chunkErr != nil {
			// Keep what we already published: the caller resumes from there
			// instead of replaying the whole gap.
			if latestPubBlock != nil {
				w.log.Error(fmt.Sprintf("Recovered blocks up to %d, then failed on range %d..%d: %v",
					latestPubBlock.GetNumber(), from, to, chunkErr))

				return latestPubBlock, nil
			}

			return nil, chunkErr
		}

		latestPubBlock = lastInChunk
	}

	if latestPubBlock == nil {
		return nil, fmt.Errorf("recovered no blocks for range %d..%d", fromBlock+1, latestNumber)
	}

	return latestPubBlock, nil
}

// recoverBlockRange fetches and publishes a single chunk, returning its last
// published block.
func (w *Feeder) recoverBlockRange(ctx context.Context, from, to int64) (*entity.EthBlock, error) {
	blocksResp, err := w.chainSrv.FetchBlocksInRange(ctx, from, to)
	if err != nil {
		return nil, fmt.Errorf("fetch blocks in range %d..%d: %w", from, to, err)
	}

	if blocksResp.Result == nil || len(*blocksResp.Result) == 0 {
		return nil, fmt.Errorf("no blocks returned for range %d..%d: %w", from, to, chain.ErrEmptyResponse)
	}

	blocks := *blocksResp.Result

	blockHashes := make([]string, 0, len(blocks))
	for i := range blocks {
		blockHashes = append(blockHashes, blocks[i].Hash)
	}

	receiptsResp, err := w.chainSrv.FetchReceipts(ctx, blockHashes)
	if err != nil {
		return nil, fmt.Errorf("could not fetch receipts: %w", err)
	}

	receiptsByBlock := make(map[string][]entity.BlockReceipt)
	if receiptsResp.Result != nil {
		for i := range *receiptsResp.Result {
			receipt := (*receiptsResp.Result)[i]
			receiptsByBlock[receipt.BlockHash] = append(receiptsByBlock[receipt.BlockHash], receipt)
		}
	}

	var latestPubBlock *entity.EthBlock
	for i := range blocks {
		block := blocks[i]

		dto := buildBlockDto(&block, receiptsByBlock[block.Hash])
		if publishErr := w.publishBlock(dto); publishErr != nil {
			// Blocks published before this one stay published — report how far
			// we got so the caller can resume from there.
			if latestPubBlock != nil {
				return latestPubBlock, nil
			}

			return nil, fmt.Errorf("could not publish block: %w", publishErr)
		}
		latestPubBlock = &block

		w.log.Info(fmt.Sprintf("Recovered block %d", dto.Number))
	}

	return latestPubBlock, nil
}

func compress(payload []byte) (*bytes.Buffer, error) {
	cPayload := &bytes.Buffer{}

	zstdWriter, err := zstd.NewWriter(cPayload)
	if err != nil {
		return nil, fmt.Errorf("could not create zstd writer: %w", err)
	}

	if _, zstdErr := zstdWriter.Write(payload); zstdErr != nil {
		zstdWriter.Close()
		return nil, zstdErr
	}

	// Close flushes the frame footer, so it has to happen before the buffer is
	// handed over — a deferred Close would run after the return.
	if closeErr := zstdWriter.Close(); closeErr != nil {
		return nil, fmt.Errorf("could not finish zstd frame: %w", closeErr)
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
	delay := max(time.Until(expectedNextBlockTime.Add(DelayNextBlock)), time.Second)

	timer.Reset(delay)
	return delay
}

func (w *Feeder) resetTimer(timer *time.Timer) {
	timer.Reset(2 * time.Second)
}

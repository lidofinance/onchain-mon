package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	databus "github.com/lidofinance/finding-forwarder/generated/databaus"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	nc "github.com/lidofinance/finding-forwarder/internal/connectors/nats"
	"github.com/lidofinance/finding-forwarder/internal/env"
	"github.com/lidofinance/finding-forwarder/internal/pkg/chain"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT, syscall.SIGKILL)
	defer stop()
	g, gCtx := errgroup.WithContext(ctx)

	cfg, envErr := env.Read("")
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log := logger.New(&cfg.AppConfig)

	transport := &http.Transport{
		MaxIdleConns:          1,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	httpClient := &http.Client{
		Transport: transport,
		Timeout:   10 * time.Second,
	}

	promStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.MetricsPrefix, cfg.AppConfig.Name, cfg.AppConfig.Env)
	promStore.BuildInfo.Inc()

	chainSrv := chain.NewChain(cfg.AppConfig.JsonRpcURL, httpClient, promStore)

	prevHashBlock := ""

	natsClient, natsErr := nc.New(&cfg.AppConfig, log)
	if natsErr != nil {
		log.Error("Could not connect to nats error:", natsErr.Error())
		return
	}
	defer natsClient.Close()

	js, jetStreamErr := jetstream.New(natsClient)
	if jetStreamErr != nil {
		log.Error("Could not connect to jetStream error:", jetStreamErr.Error())
		return
	}

	log.Info(fmt.Sprintf(`started %s`, cfg.AppConfig.Name))
	ticker := time.NewTicker(6 * time.Second)
	defer ticker.Stop()

	g.Go(func() error {
		for {
			select {
			case <-gCtx.Done():
				return gCtx.Err()
			case <-ticker.C:
				block, err := chainSrv.GetLatestBlock(gCtx)
				if err != nil {
					log.Error("GetLatestBlock error:", err.Error())
					continue
				}

				if block.Result.Hash == prevHashBlock {
					continue
				}

				blockReceipts, err := chainSrv.GetBlockReceipts(gCtx, block.Result.Hash)
				if err != nil {
					log.Error("GetBlockReceipts error:", err.Error())
					continue
				}

				var receipts []databus.BlockDtoJsonReceiptsElem
				for _, receipt := range *blockReceipts.Result {
					txLog := databus.BlockDtoJsonReceiptsElem{}
					if receipt.To != "" {
						txLog.To = &receipt.To
					}

					logsElem := make([]databus.BlockDtoJsonReceiptsElemLogsElem, 0, len(receipt.Logs))
					for _, receiptLog := range receipt.Logs {

						blockNumber, _ := strconv.ParseUint(receiptLog.BlockNumber, 0, 64)
						logIndex, _ := strconv.ParseUint(receiptLog.LogIndex, 0, 64)
						trxInd, _ := strconv.ParseUint(receiptLog.TransactionIndex, 0, 64)

						logsElem = append(logsElem, databus.BlockDtoJsonReceiptsElemLogsElem{
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
						Logs: logsElem,
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
					log.Error(fmt.Sprintf(`Could not marshal blockDto %s`, marshalErr))
					continue
				}

				prevHashBlock = block.Result.Hash
				if _, publishErr := js.PublishAsync(`blocks.l1`, payload, jetstream.WithMsgID(blockDto.Hash)); publishErr != nil {
					promStore.PublishedBlocks.With(prometheus.Labels{metrics.Status: metrics.StatusFail}).Inc()
					log.Error(fmt.Sprintf("could not publish block %d to JetStream: error: %v", blockDto.Number, publishErr))
					continue
				}

				log.Info(fmt.Sprintf(`%d, %s`, blockDto.Number, blockDto.Hash))
				promStore.PublishedAlerts.With(prometheus.Labels{metrics.Status: metrics.StatusOk}).Inc()
			}
		}
	})

	g.Go(func() error {
		<-gCtx.Done()
		return nil
	})

	if err := g.Wait(); err != nil && !errors.Is(err, context.Canceled) {
		log.Error("g.Wait error:", err.Error())
	}

	log.Info(`Feeder done`)
}

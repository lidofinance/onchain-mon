package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/sync/errgroup"

	server "github.com/lidofinance/finding-forwarder/internal/app/http_server"
	"github.com/lidofinance/finding-forwarder/internal/connectors/logger"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
	redisConnector "github.com/lidofinance/finding-forwarder/internal/connectors/redis"
	"github.com/lidofinance/finding-forwarder/internal/env"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	cfg, envErr := env.Read("")
	if envErr != nil {
		fmt.Println("Read env error:", envErr.Error())
		return
	}

	log, logErr := logger.New(&cfg.AppConfig)
	if logErr != nil {
		fmt.Println("Logger error:", logErr.Error())
		return
	}

	red := redisConnector.New(&cfg.AppConfig)

	log.Info(fmt.Sprintf(`started %s application`, cfg.AppConfig.Name))

	r := chi.NewRouter()
	promStore := metrics.New(prometheus.NewRegistry(), cfg.AppConfig.Name, cfg.AppConfig.Env)

	usecase := server.Usecase(&cfg.AppConfig)

	app := server.New(log, promStore, usecase, red)

	app.Metrics.BuildInfo.Inc()
	app.RegisterRoutes(r)

	g, gCtx := errgroup.WithContext(ctx)

	app.RunHTTPServer(gCtx, g, cfg.AppConfig.Port, r)
	// someDaemon(gCtx, g, red)

	if err := g.Wait(); err != nil {
		log.Error(err)
	}

	fmt.Println(`Main done`)
}

/*func someDaemon(gCtx context.Context, g *errgroup.Group, redisClient *redis.Client) {
	g.Go(func() error {
		for {
			select {
			case <-time.After(1 * time.Second):

				// Get the message
				body, err := redisClient.LIndex(gCtx, topic, - 1).Bytes()
				iferr ! =nil && !errors.Is(err, redis.Nil) {
				return err
				}
				// No message, sleep for a while
				if errors.Is(err, redis.Nil) {
					time.Sleep(time.Second)
					continue
				}
				// Process the message
				err = h(&Msg{
					Topic: topic,
					Body:  body,
				})
				iferr ! =nil {
				continue
			}
				// If the processing succeeds, delete the message
				iferr := q.client.RPop(ctx, topic).Err(); err ! =nil {
				return err
			}

			case <-gCtx.Done():
				return nil
			}
		}
	})
}*/

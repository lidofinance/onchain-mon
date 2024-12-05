package consumer

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

const (
	Telegram = `Telegram`
	Discord  = `Discord`
	OpsGenie = `OpsGenie`
)

type Consumer struct {
	log         *slog.Logger
	metrics     *metrics.Store
	cache       *expirable.LRU[string, uint]
	redisClient *redis.Client
	repo        *Repo

	name        string
	subject     string
	severitySet registry.FindingMapping
	byQuorum    bool
	quorumSize  uint
	notifier    notifiler.FindingSender
}

func New(
	log *slog.Logger,
	metrics *metrics.Store,
	cache *expirable.LRU[string, uint],
	redisClient *redis.Client,
	repo *Repo,
	consumerName,
	subject string,
	SeveritySet registry.FindingMapping,
	byQuorum bool,
	quorumSize uint,
	notifier notifiler.FindingSender,
) *Consumer {
	return &Consumer{
		log:         log,
		metrics:     metrics,
		cache:       cache,
		redisClient: redisClient,
		repo:        repo,

		name:        consumerName,
		subject:     subject,
		severitySet: SeveritySet,
		byQuorum:    byQuorum,
		quorumSize:  quorumSize,
		notifier:    notifier,
	}
}

func NewConsumers(log *slog.Logger, metrics *metrics.Store, redisClient *redis.Client, repo *Repo, quorumSize uint, cfg *env.NotificationConfig, notificationChannels *env.NotificationChannels) ([]*Consumer, error) {
	var consumers []*Consumer

	for _, consumerCfg := range cfg.Consumers {
		for _, subject := range consumerCfg.Subjects {
			parts := strings.Split(subject, ".")
			if len(parts) < 3 {
				return nil, fmt.Errorf("invalid subject format: %s", subject)
			}
			teamName := parts[1]
			botName := parts[2]

			consumerName := fmt.Sprintf("%s_%s_%s", teamName, consumerCfg.ConsumerName, botName)

			var notificationChannel notifiler.FindingSender
			switch consumerCfg.Type {
			case Telegram:
				channel, exists := notificationChannels.TelegramChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("telegram channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			case Discord:
				channel, exists := notificationChannels.DiscordChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("discord channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			case OpsGenie:
				channel, exists := notificationChannels.OpsGenieChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("opsgenie channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			default:
				return nil, fmt.Errorf("unsupported consumer type '%s'", consumerCfg.Type)
			}

			const LruSize = 125
			cache := expirable.NewLRU[string, uint](LruSize, nil, time.Minute*10)

			consumer := New(
				log, metrics, cache, redisClient, repo,
				consumerName,
				subject,
				consumerCfg.SeveritySet,
				consumerCfg.ByQuorum,
				quorumSize,
				notificationChannel,
			)

			consumers = append(consumers, consumer)
		}
	}

	return consumers, nil
}

var statusTemplate = "%s:finding:%s:status"
var countTemplate = "%s:finding:%s:count"

type Status string

const (
	StatusNotSend Status = "not_send"
	StatusSending Status = "sending"
	StatusSent    Status = "sent"
)

const (
	TTLMins10 = 10 * time.Minute
	TTLMin1   = 1 * time.Minute
)

func (c *Consumer) GetName() string {
	return c.name
}

func (c *Consumer) GetTopic() string {
	return c.subject
}

func (c *Consumer) GetConsumeHandler(ctx context.Context) func(msg jetstream.Msg) {
	return func(msg jetstream.Msg) {
		finding := new(databus.FindingDtoJson)

		if alertErr := json.Unmarshal(msg.Data(), finding); alertErr != nil {
			c.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
			c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
			c.terminateMessage(msg)
			return
		}

		defer func() {
			finding = nil
		}()

		if _, ok := c.severitySet[finding.Severity]; !ok {
			c.ackMessage(msg)
			return
		}

		if c.byQuorum == false {
			if sendErr := c.notifier.SendFinding(ctx, finding); sendErr != nil {
				c.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr),
					slog.Attr{
						Key:   "alertID",
						Value: slog.StringValue(finding.AlertId),
					})
				c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}

			msgInfo := fmt.Sprintf("%s: %s-%s set message without quorum", c.name, finding.BotName, finding.AlertId)
			if finding.BlockNumber != nil {
				msgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
			}

			c.log.Info(msgInfo,
				slog.Attr{
					Key:   `desc`,
					Value: slog.StringValue(finding.Description),
				},
				slog.Attr{
					Key:   `name`,
					Value: slog.StringValue(finding.Name),
				},
				slog.Attr{
					Key:   `alertId`,
					Value: slog.StringValue(finding.AlertId),
				},
				slog.Attr{
					Key:   `severity`,
					Value: slog.StringValue(string(finding.Severity)),
				},
			)

			c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
			c.ackMessage(msg)
			return
		}

		key := finding.UniqueKey
		countKey := fmt.Sprintf(countTemplate, c.name, key)
		statusKey := fmt.Sprintf(statusTemplate, c.name, key)

		var (
			count uint64
			err   error
		)

		if !c.cache.Contains(countKey) {
			c.cache.Add(countKey, uint(1))

			count, err = c.redisClient.Incr(ctx, countKey).Uint64()
			if err != nil {
				c.log.Error(fmt.Sprintf(`Could not increase key value: %v`, err))
				c.metrics.RedisErrors.Inc()
				c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}

			if count == 1 {
				if err := c.redisClient.Expire(ctx, countKey, TTLMins10).Err(); err != nil {
					c.log.Error(fmt.Sprintf(`Could not set expire time: %v`, err))
					c.metrics.RedisErrors.Inc()
					c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()

					if _, err := c.redisClient.Decr(ctx, countKey).Result(); err != nil {
						c.metrics.RedisErrors.Inc()
						c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
						c.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
					}

					c.nackMessage(msg)
					return
				}
			}
		} else {
			v, _ := c.cache.Get(countKey)
			c.cache.Add(countKey, v+1)

			count, err = c.redisClient.Get(ctx, countKey).Uint64()
			if err != nil {
				c.log.Error(fmt.Sprintf(`Could not get key value: %v`, err))
				c.metrics.RedisErrors.Inc()
				c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}
		}

		touchTimes, _ := c.cache.Get(countKey)

		msgInfo := fmt.Sprintf("%s: %s-%s read %d times. %s...%s", c.name, finding.BotName, finding.AlertId, touchTimes, key[0:4], key[len(key)-4:])
		if finding.BlockNumber != nil {
			msgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
		}

		c.log.Info(msgInfo,
			slog.Attr{
				Key:   `desc`,
				Value: slog.StringValue(finding.Description),
			},
			slog.Attr{
				Key:   `name`,
				Value: slog.StringValue(finding.Name),
			},
			slog.Attr{
				Key:   `alertId`,
				Value: slog.StringValue(finding.AlertId),
			},
			slog.Attr{
				Key:   `severity`,
				Value: slog.StringValue(string(finding.Severity)),
			},
			slog.Attr{
				Key:   `hash`,
				Value: slog.StringValue(key),
			},
		)

		if uint(count) >= c.quorumSize {
			status, err := c.repo.GetStatus(ctx, statusKey)
			if err != nil {
				c.log.Error(fmt.Sprintf(`Could not get notification status: %v`, err))
				c.metrics.RedisErrors.Inc()
				c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}

			if status == StatusSending {
				c.log.Info(fmt.Sprintf("Another instance is sending finding: %s", finding.AlertId))
				return
			}

			if status == StatusSent {
				c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
				c.ackMessage(msg)

				c.cache.Remove(countKey)

				if err := c.redisClient.Expire(ctx, countKey, TTLMin1).Err(); err != nil {
					c.log.Error(fmt.Sprintf(`Could not set expire time: %v`, err))
					c.metrics.RedisErrors.Inc()
				}

				if err := c.redisClient.Expire(ctx, statusKey, TTLMin1).Err(); err != nil {
					c.log.Error(fmt.Sprintf(`Could not set expire time: %v`, err))
					c.metrics.RedisErrors.Inc()
				}

				c.log.Info(fmt.Sprintf("Another instance already sent finding: %s", finding.AlertId))
				return
			}

			if status == StatusNotSend {
				readyToSend, err := c.repo.SetSendingStatus(ctx, countKey, statusKey)
				if err != nil {
					c.log.Error(fmt.Sprintf(`Could not check notification status for AlertID: %s: %v`, finding.AlertId, err))
					c.metrics.RedisErrors.Inc()
					c.nackMessage(msg)
					return
				}

				if readyToSend {
					if sendErr := c.notifier.SendFinding(ctx, finding); sendErr != nil {
						c.log.Error(fmt.Sprintf(`Could not send bot-finding: %v`, sendErr), slog.Attr{
							Key:   "alertID",
							Value: slog.StringValue(finding.AlertId),
						})

						count, err := c.redisClient.Decr(ctx, countKey).Result()
						if err != nil {
							c.metrics.RedisErrors.Inc()
							c.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
						} else if count <= 0 {
							if err = c.redisClient.Del(ctx, countKey).Err(); err != nil {
								c.metrics.RedisErrors.Inc()
								c.log.Error(fmt.Sprintf(`Could not delete countKey %s: %v`, countKey, err))
							}
						}

						if err = c.redisClient.Del(ctx, statusKey).Err(); err != nil {
							c.metrics.RedisErrors.Inc()
							c.log.Error(fmt.Sprintf(`Could not delete statusKey %s: %v`, statusKey, err))
						}

						c.cache.Remove(countKey)

						c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
						c.nackMessage(msg)
						return
					}

					c.metrics.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
					c.ackMessage(msg)

					c.log.Info(fmt.Sprintf("%s sent finding to %s %s.%s", c.name, c.notifier.GetType(), finding.BotName, finding.AlertId),
						slog.Attr{
							Key:   `alertId`,
							Value: slog.StringValue(finding.AlertId),
						},
						slog.Attr{
							Key:   `name`,
							Value: slog.StringValue(finding.Name),
						},
						slog.Attr{
							Key:   `desc`,
							Value: slog.StringValue(finding.Description),
						},
					)

					if err := c.repo.SeStatusSent(ctx, statusKey); err != nil {
						c.metrics.RedisErrors.Inc()
						c.log.Error(fmt.Sprintf(`Could not set notification StatusSent: %s`, err.Error()))
					}
				}
			}
		}
	}
}

func (c *Consumer) terminateMessage(msg jetstream.Msg) {
	if termErr := msg.Term(); termErr != nil {
		c.log.Error(fmt.Sprintf(`Could not term msg: %v`, termErr))
	}
}

func (c *Consumer) nackMessage(msg jetstream.Msg) {
	if nackErr := msg.Nak(); nackErr != nil {
		c.log.Error(fmt.Sprintf(`Could not nack msg: %v`, nackErr))
	}
}

func (c *Consumer) ackMessage(msg jetstream.Msg) {
	if ackErr := msg.Ack(); ackErr != nil {
		c.log.Error(fmt.Sprintf(`Could not ack msg: %v`, ackErr))
	}
}

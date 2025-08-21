package consumer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/env"
	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type Consumer struct {
	log         *slog.Logger
	mtrs        *metrics.Store
	cache       *expirable.LRU[string, uint]
	redisClient *redis.Client
	repo        *Repo

	instance         string
	name             string
	subject          string
	severitySet      registry.FindingMapping
	byQuorum         bool
	quorumSize       uint
	findingFilterMap registry.FindingFilterMap
	notifier         notifiler.FindingSender
}

const LRUCacheExpiration = time.Minute * 10

func New(
	log *slog.Logger,
	mtrs *metrics.Store,
	cache *expirable.LRU[string, uint],
	redisClient *redis.Client,
	repo *Repo,
	instance string,
	consumerName,
	subject string,
	severitySet registry.FindingMapping,
	findingFilterMap registry.FindingFilterMap,
	byQuorum bool,
	quorumSize uint,
	notifier notifiler.FindingSender,
) *Consumer {
	return &Consumer{
		log:         log,
		mtrs:        mtrs,
		cache:       cache,
		redisClient: redisClient,
		repo:        repo,

		instance:         instance,
		name:             consumerName,
		subject:          subject,
		severitySet:      severitySet,
		findingFilterMap: findingFilterMap,
		byQuorum:         byQuorum,
		quorumSize:       quorumSize,
		notifier:         notifier,
	}
}

func NewConsumers(
	log *slog.Logger,
	mtr *metrics.Store,
	redisClient *redis.Client,
	instance string,
	repo *Repo,
	quorumSize uint,
	cfg *env.NotificationConfig,
	notificationChannels *env.NotificationChannels,
) ([]*Consumer, error) {
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
			case registry.Telegram:
				channel, exists := notificationChannels.TelegramChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("telegram channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			case registry.Discord:
				channel, exists := notificationChannels.DiscordChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("discord channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			case registry.OpsGenie:
				channel, exists := notificationChannels.OpsGenieChannels[consumerCfg.ChannelID]
				if !exists {
					return nil, fmt.Errorf("opsgenie channel with id '%s' not found for consumer '%s'", consumerCfg.ChannelID, consumerCfg.ConsumerName)
				}
				notificationChannel = channel
			default:
				return nil, fmt.Errorf("unsupported consumer type '%s'", consumerCfg.Type)
			}

			const LruSize = 125
			cache := expirable.NewLRU[string, uint](LruSize, nil, LRUCacheExpiration)

			consumer := New(
				log, mtr, cache, redisClient, repo,
				instance,
				consumerName,
				subject,
				consumerCfg.SeveritySet,
				consumerCfg.FindingFilterMap,
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
	TTLMins30 = 30 * time.Minute
	TTLMin1   = 1 * time.Minute
)

const NackDelayMsg = 3 * time.Second

func (c *Consumer) GetName() string {
	return c.name
}

func (c *Consumer) GetTopic() string {
	return c.subject
}

func (c *Consumer) ByQuorum() bool {
	return c.byQuorum
}

func getCoolDownKey(botName, alertId, alertBody, natsConsumerName string) string {
	return fmt.Sprintf("%s_%s_%s_%s", botName, alertId, alertBody, natsConsumerName)
}

func (c *Consumer) GetConsumeHandler(ctx context.Context) func(msg jetstream.Msg) {
	return func(msg jetstream.Msg) {
		finding := new(databus.FindingDtoJson)

		if alertErr := json.Unmarshal(msg.Data(), finding); alertErr != nil {
			c.log.Error(fmt.Sprintf(`Broken message: %v`, alertErr))
			c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
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

		if len(c.findingFilterMap) > 0 {
			if _, ok := c.findingFilterMap[finding.AlertId]; !ok {
				c.ackMessage(msg)
				return
			}
		}

		if !c.byQuorum {
			_, putOnStreamErr := c.repo.AddIntoStream(ctx, finding, c.notifier, c.instance, false)
			if putOnStreamErr != nil {
				c.logError(fmt.Sprintf(`Could not push debug-fidning into redis queue: %v`, putOnStreamErr), finding)

				c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackDelayMessage(msg, NackDelayMsg)
				return
			}

			msgInfo := fmt.Sprintf("%s: put %s into redis-debug-queue", c.instance, finding.AlertId)
			if finding.BlockNumber != nil {
				msgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
			}

			c.log.Info(
				msgInfo,
				slog.String("alertId", finding.AlertId),
				slog.String("name", finding.Name),
				slog.String("desc", finding.Description),
				slog.String("setBy", c.instance),
				slog.String("consumer", c.name),
				slog.String("bot-name", finding.BotName),
				slog.String("severity", string(finding.Severity)),
				slog.String("uniqueKey", finding.UniqueKey),
			)

			c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
			c.ackMessage(msg)
			return
		}

		hash := sha256.Sum256([]byte(finding.Description))
		bodyDesc := hex.EncodeToString(hash[:])

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
				c.logError(fmt.Sprintf(`Could not increase key value: %v`, err), finding)

				c.mtrs.RedisErrors.Inc()
				c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}

			if count == 1 {
				if err := c.redisClient.Expire(ctx, countKey, TTLMins10).Err(); err != nil {
					c.logError(fmt.Sprintf(`Could not set expire time: %v`, err), finding)

					c.mtrs.RedisErrors.Inc()
					c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()

					if _, err := c.redisClient.Decr(ctx, countKey).Result(); err != nil {
						c.logError(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err), finding)
						c.mtrs.RedisErrors.Inc()
						c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
					}
				}
				c.nackMessage(msg)
				return
			}
		} else {
			v, _ := c.cache.Get(countKey)
			c.cache.Add(countKey, v+1)

			count, err = c.redisClient.Get(ctx, countKey).Uint64()
			if err != nil {
				if errors.Is(err, redis.Nil) {
					c.cache.Remove(countKey)
					c.log.Warn(fmt.Sprintf(`Key(%s) is expired`, countKey))
					c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
					c.ackMessage(msg)
					return
				}

				c.logError(fmt.Sprintf(`Could not get key(%s) value: %v`, countKey, err), finding)
				c.mtrs.RedisErrors.Inc()
				c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}
		}

		touchTimes, _ := c.cache.Get(countKey)

		msgInfo := fmt.Sprintf("%s: %s-%s read %d times. %s...%s",
			c.name, finding.BotName, finding.AlertId, touchTimes, key[0:4], key[len(key)-4:],
		)
		if finding.BlockNumber != nil {
			msgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
		}

		c.log.Info(msgInfo,
			slog.String("alertId", finding.AlertId),
			slog.String("name", finding.Name),
			slog.String("desc", finding.Description),
			slog.String("setBy", c.instance),
			slog.String("consumer", c.name),
			slog.String("bot-name", finding.BotName),
			slog.String("severity", string(finding.Severity)),
			slog.String("uniqueKey", finding.UniqueKey),
		)

		if uint(count) >= c.quorumSize {
			status, err := c.repo.GetStatus(ctx, statusKey)
			if err != nil {
				c.logError(fmt.Sprintf(`Could not get notification status: %v`, err), finding)

				c.mtrs.RedisErrors.Inc()
				c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusFail}).Inc()
				c.nackMessage(msg)
				return
			}

			if status == StatusSending {
				c.log.Info(fmt.Sprintf("%s[%s] - another instance is sending finding: %s", c.instance, c.notifier.GetType(), finding.AlertId))
				c.nackMessage(msg)
				return
			}

			if status == StatusSent {
				c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
				c.ackMessage(msg)

				c.cache.Remove(countKey)

				if err := c.redisClient.Expire(ctx, countKey, TTLMin1).Err(); err != nil {
					c.logError(fmt.Sprintf(`Could not set expire time for countKey: %v`, err), finding)
					c.mtrs.RedisErrors.Inc()
				}

				if err := c.redisClient.Expire(ctx, statusKey, TTLMin1).Err(); err != nil {
					c.logError(fmt.Sprintf(`Could not set expire time: %v for statusKey`, err), finding)
					c.mtrs.RedisErrors.Inc()
				}

				c.log.Info(fmt.Sprintf("%s[%s] - another instance already sent finding: %s", c.instance, c.notifier.GetType(), finding.AlertId))
				return
			}

			if status == StatusNotSend {
				// same alert by content but may different blockNumber
				isCooldownActive, coolDownErr := c.repo.GetCoolDown(ctx, getCoolDownKey(finding.BotName, finding.AlertId, bodyDesc, c.name))
				if coolDownErr != nil {
					c.logError(fmt.Sprintf(`Could not get cool-down status: %v`, coolDownErr), finding)
					c.mtrs.RedisErrors.Inc()
				}

				if isCooldownActive {
					c.log.Info(fmt.Sprintf("Got isCooldownActive by %s", finding.AlertId))
					c.ackMessage(msg)
					return
				}

				readyToSend, err := c.repo.SetSendingStatus(ctx, countKey, statusKey)
				if err != nil {
					c.logError(fmt.Sprintf(`Could not check notification status for AlertID: %s: %v`, finding.AlertId, err), finding)

					c.mtrs.RedisErrors.Inc()
					c.nackMessage(msg)
					return
				}

				if readyToSend {
					// Sends via notification channel {Tg, Discord, OpsGenia}
					if sendErr := c.notifier.SendFinding(ctx, finding, c.instance); sendErr != nil {
						// When we found 429 - put finding into redis-queue for delayed sending
						if errors.Is(sendErr, notifiler.ErrRateLimited) {
							_, putOnStreamErr := c.repo.AddIntoStream(ctx, finding, c.notifier, c.instance, true)
							if putOnStreamErr != nil {
								c.logError(fmt.Sprintf(`Could not push msg into redis queue: %v`, putOnStreamErr), finding)
								c.failAndNack(ctx, msg, countKey, statusKey)
								return
							}

							quorumMsgInfo := fmt.Sprintf("%s pushed quorum-finding into %s stream %s[%s]",
								c.instance, c.notifier.GetChannelID(), finding.BotName, finding.AlertId,
							)
							if finding.BlockNumber != nil {
								quorumMsgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
							}

							c.log.Info(
								quorumMsgInfo,
								slog.String("alertId", finding.AlertId),
								slog.String("name", finding.Name),
								slog.String("desc", finding.Description),
								slog.String("setBy", c.instance),
								slog.String("consumer", c.name),
								slog.String("bot-name", finding.BotName),
								slog.String("severity", string(finding.Severity)),
								slog.String("uniqueKey", finding.UniqueKey),
							)
						} else {
							c.logError(fmt.Sprintf(`Could not send quorum-finding: %v`, sendErr), finding)
							c.failAndNack(ctx, msg, countKey, statusKey)
							return
						}
					} else {
						quorumMsgInfo := fmt.Sprintf("%s[%s] send finding %s[%s]", c.instance, c.notifier.GetType(), finding.BotName, finding.AlertId)
						if finding.BlockNumber != nil {
							quorumMsgInfo += fmt.Sprintf(" blockNumber %d", *finding.BlockNumber)
						}

						c.log.Info(
							quorumMsgInfo,
							slog.String("alertId", finding.AlertId),
							slog.String("name", finding.Name),
							slog.String("desc", finding.Description),
							slog.String("setBy", c.instance),
							slog.String("consumer", c.name),
							slog.String("bot-name", finding.BotName),
							slog.String("severity", string(finding.Severity)),
							slog.String("uniqueKey", finding.UniqueKey),
						)
					}

					c.mtrs.SentAlerts.With(prometheus.Labels{metrics.ConsumerName: c.name, metrics.Status: metrics.StatusOk}).Inc()
					c.ackMessage(msg)

					if err := c.repo.SeStatusSent(ctx, statusKey); err != nil {
						c.logError(fmt.Sprintf(`Could not set notification StatusSent: %s`, err.Error()), finding)
						c.mtrs.RedisErrors.Inc()
					}

					if err := c.repo.SetCoolDown(ctx, getCoolDownKey(finding.BotName, finding.AlertId, bodyDesc, c.name)); err != nil {
						c.logError(fmt.Sprintf(`Could not set cool down status: %s`, err.Error()), finding)
						c.mtrs.RedisErrors.Inc()
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

func (c *Consumer) nackDelayMessage(msg jetstream.Msg, delay time.Duration) {
	if nackErr := msg.NakWithDelay(delay); nackErr != nil {
		c.log.Error(fmt.Sprintf(`Could not nack with delay msg: %v`, nackErr))
	}
}

func (c *Consumer) ackMessage(msg jetstream.Msg) {
	if ackErr := msg.Ack(); ackErr != nil {
		c.log.Error(fmt.Sprintf(`Could not ack msg: %v`, ackErr))
	}
}

func (c *Consumer) failAndNack(
	ctx context.Context,
	msg jetstream.Msg,
	countKey, statusKey string,
) {
	if count, err := c.redisClient.Decr(ctx, countKey).Result(); err != nil {
		c.mtrs.RedisErrors.Inc()
		c.log.Error(fmt.Sprintf(`Could not decrease count key %s: %v`, countKey, err))
	} else if count <= 0 {
		if err := c.redisClient.Del(ctx, countKey).Err(); err != nil {
			c.mtrs.RedisErrors.Inc()
			c.log.Error(fmt.Sprintf(`Could not delete countKey %s: %v`, countKey, err))
		}
	}

	if err := c.redisClient.Del(ctx, statusKey).Err(); err != nil {
		c.mtrs.RedisErrors.Inc()
		c.log.Error(fmt.Sprintf(`Could not delete statusKey %s: %v`, statusKey, err))
	}

	c.cache.Remove(countKey)

	c.mtrs.SentAlerts.With(prometheus.Labels{
		metrics.ConsumerName: c.name,
		metrics.Status:       metrics.StatusFail,
	}).Inc()

	c.nackMessage(msg)
}

func (c *Consumer) logError(errMsg string, finding *databus.FindingDtoJson) {
	c.log.Error(errMsg,
		slog.String("alertId", finding.AlertId),
		slog.String("name", finding.Name),
		slog.String("desc", finding.Description),
		slog.String("setBy", c.instance),
		slog.String("consumer", c.name),
		slog.String("bot-name", finding.BotName),
		slog.String("severity", string(finding.Severity)),
		slog.String("uniqueKey", finding.UniqueKey),
	)
}

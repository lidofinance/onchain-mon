package notifiler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type Telegram struct {
	botToken               string
	chatID                 string
	httpClient             *http.Client
	metrics                *metrics.Store
	blockExplorer          string
	channelID              string
	redisStreamName        string
	redisConsumerGroupName string
}

func NewTelegram(botToken, chatID string,
	httpClient *http.Client, metricsStore *metrics.Store,
	blockExplorer, channelID,
	redisStreamName, redisConsumerGroupName string,
) *Telegram {
	return &Telegram{
		botToken:               botToken,
		chatID:                 chatID,
		httpClient:             httpClient,
		metrics:                metricsStore,
		blockExplorer:          blockExplorer,
		channelID:              channelID,
		redisStreamName:        redisStreamName,
		redisConsumerGroupName: redisConsumerGroupName,
	}
}

const MaxTelegramMessageLength = 4096
const WarningTelegramMessage = "Warn: Msg >=4096, pls review description message"
const TelegramLabel = `telegram`

func (t *Telegram) SendFinding(ctx context.Context, alert *databus.FindingDtoJson, quorumBy string) error {
	message := TruncateMessageWithAlertID(
		fmt.Sprintf("%s\n\n%s", alert.Name, FormatAlert(alert, quorumBy, t.blockExplorer)),
		MaxTelegramMessageLength,
		WarningTelegramMessage,
	)

	if alert.Severity != databus.SeverityUnknown {
		m := escapeMarkdownV1(message)

		if sendErr := t.send(ctx, m, true); sendErr != nil {
			message += "\n\nWarning: Could not send msg as markdown"
			return t.send(ctx, message, false)
		}

		return nil
	}

	return t.send(ctx, message, false)
}

func (t *Telegram) send(ctx context.Context, message string, useMarkdown bool) error {
	requestURL := fmt.Sprintf(
		"https://api.telegram.org/bot%s/sendMessage?disable_web_page_preview=true&disable_notification=true&chat_id=-%s&text=%s",
		t.botToken,
		t.chatID,
		url.QueryEscape(message),
	)
	if useMarkdown {
		requestURL += `&parse_mode=markdown`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("could not create telegram request: %w", err)
	}

	start := time.Now()

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send telegram request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		t.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: TelegramLabel}).Observe(duration)
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		t.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusFail}).Inc()
		return ErrRateLimited
	}

	if resp.StatusCode != http.StatusOK {
		t.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusFail}).Inc()
		return fmt.Errorf("received from telegram non-200 response code: %v", resp.Status)
	}

	t.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusOk}).Inc()
	return nil
}

func (t *Telegram) GetType() registry.NotificationChannel {
	return registry.Telegram
}
func (t *Telegram) GetChannelID() string {
	return t.channelID
}
func (t *Telegram) GetRedisStreamName() string {
	return fmt.Sprintf("%s:%s", t.redisStreamName, t.channelID)
}
func (t *Telegram) GetRedisConsumerGroupName() string {
	return fmt.Sprintf("%s:%s", t.redisConsumerGroupName, t.channelID)
}

// Telegram supports two versions of markdown. V1, V2
// For V1 we have to escape some symbols
//
// V2 - is more reach for special symbols, more you can find by link
// https://core.telegram.org/bots/update56kabdkb12ibuisabdubodbasbdaosd#markdownv2-style
func escapeMarkdownV1(input string) string {
	specialChars := map[string]struct{}{
		`_`: {},
	}

	var escaped strings.Builder
	for _, char := range input {
		if _, ok := specialChars[string(char)]; ok {
			escaped.WriteString(`\`)
		}

		escaped.WriteRune(char)
	}

	return escaped.String()
}

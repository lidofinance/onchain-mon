package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

// See https://docs.slack.dev/messaging/sending-messages-using-incoming-webhooks/

type Slack struct {
	webhookURL    string
	httpClient    *http.Client
	metrics       *metrics.Store
	blockExplorer string
	source        string
}

type slackMessagePayload struct {
	Text     string `json:"text"`
	Markdown bool   `json:"mrkdwn"`
}

const SlackLabel = `slack`

func NewSlack(
	webhookURL string,
	httpClient *http.Client,
	metricsStore *metrics.Store,
	source, blockExplorer string,
) *Slack {
	return &Slack{
		webhookURL:    webhookURL,
		httpClient:    httpClient,
		metrics:       metricsStore,
		blockExplorer: blockExplorer,
		source:        source,
	}
}

const SlackRetryAfter = 10 * time.Second
const MaxSlackMsgLength = 3000
const WarningSlackMessage = "Warn: Msg >=3000, pls review description message"

func (s *Slack) SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error {
	formatted := NormalizeMarkdownForSlack(FormatAlert(alert, s.source, s.blockExplorer))
	message := TruncateMessageWithAlertID(
		fmt.Sprintf("%s\n\n%s", alert.Name, formatted),
		MaxSlackMsgLength,
		WarningSlackMessage,
	)
	return s.send(ctx, message)
}

func (s *Slack) send(ctx context.Context, message string) error {
	payload := slackMessagePayload{Text: message, Markdown: true}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal Slack payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating Slack request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send Slack request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
		duration := time.Since(start).Seconds()
		s.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: SlackLabel}).Observe(duration)
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		s.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: SlackLabel, metrics.Status: metrics.StatusFail}).Inc()
		if v := resp.Header.Get("Retry-After"); v != "" {
			if resetAfter, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return &RateLimitedError{ResetAfter: time.Duration(int(resetAfter)) * time.Second, Err: ErrRateLimited}
			}
			return &RateLimitedError{ResetAfter: SlackRetryAfter, Err: ErrRateLimited}
		}
		return &RateLimitedError{ResetAfter: SlackRetryAfter, Err: ErrRateLimited}
	}

	if resp.StatusCode != http.StatusOK {
		s.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: SlackLabel, metrics.Status: metrics.StatusFail}).Inc()
		return fmt.Errorf("received from Slack non-200 response code: %v", resp.Status)
	}

	s.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: SlackLabel, metrics.Status: metrics.StatusOk}).Inc()
	return nil
}

func (s *Slack) GetType() registry.NotificationChannel {
	return registry.Slack
}

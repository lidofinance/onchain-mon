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

type Discord struct {
	webhookURL    string
	httpClient    *http.Client
	metrics       *metrics.Store
	blockExplorer string
}

type MessagePayload struct {
	Content string `json:"content"`
}

const DiscordLabel = `discord`

func NewDiscord(
	webhookURL string,
	httpClient *http.Client,
	metricsStore *metrics.Store,
	blockExplorer string,
) *Discord {
	return &Discord{
		webhookURL:    webhookURL,
		httpClient:    httpClient,
		metrics:       metricsStore,
		blockExplorer: blockExplorer,
	}
}

const MaxDiscordMsgLength = 2000
const WarningDiscordMessage = "Warn: Msg >=2000, pls review description message"
const DiscordRetryAfter = 10 * time.Second

func (d *Discord) SendFinding(ctx context.Context, alert *databus.FindingDtoJson, quorumBy string) error {
	message := TruncateMessageWithAlertID(
		fmt.Sprintf("%s\n\n%s", alert.Name, FormatAlert(alert, quorumBy, d.blockExplorer)),
		MaxDiscordMsgLength,
		WarningDiscordMessage,
	)

	return d.send(ctx, message)
}

func (d *Discord) send(ctx context.Context, message string) error {
	payload := MessagePayload{
		Content: message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal Discord payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating Discord request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	start := time.Now()
	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send Discord request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		d.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: DiscordLabel}).Observe(duration)
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		d.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: DiscordLabel, metrics.Status: metrics.StatusFail}).Inc()
		if v := resp.Header.Get("X-RateLimit-Reset-After"); v != "" {
			resetAfter, err := strconv.ParseFloat(strings.TrimSpace(v), 64)
			if err != nil {
				return &RateLimitedError{
					ResetAfter: DiscordRetryAfter,
					Err:        ErrRateLimited,
				}
			}

			return &RateLimitedError{
				ResetAfter: time.Duration(int(resetAfter)) * time.Second,
				Err:        ErrRateLimited,
			}
		}

		return &RateLimitedError{
			ResetAfter: DiscordRetryAfter,
			Err:        ErrRateLimited,
		}
	}

	if resp.StatusCode != http.StatusNoContent {
		d.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: DiscordLabel, metrics.Status: metrics.StatusFail}).Inc()
		return fmt.Errorf("received from Discord non-204 response code: %v", resp.Status)
	}

	d.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: DiscordLabel, metrics.Status: metrics.StatusOk}).Inc()
	return nil
}

func (d *Discord) GetType() registry.NotificationChannel {
	return registry.Discord
}

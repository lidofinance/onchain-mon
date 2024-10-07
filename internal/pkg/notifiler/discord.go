package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/generated/databus"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

type discord struct {
	webhookURL string
	httpClient *http.Client
	metrics    *metrics.Store
	source     string
}

type MessagePayload struct {
	Content string `json:"content"`
}

func NewDiscord(webhookURL string, httpClient *http.Client, metricsStore *metrics.Store, source string) *discord {
	return &discord{
		webhookURL: webhookURL,
		httpClient: httpClient,
		metrics:    metricsStore,
		source:     source,
	}
}

const maxDiscordMsgLength = 2000
const warningDiscordMessage = "Warn: Msg >=2000, pls review description message"

func (d *discord) SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error {
	message := TruncateMessageWithAlertID(
		fmt.Sprintf("%s\n\n%s", alert.Name, FormatAlert(alert, d.source)),
		maxDiscordMsgLength,
		warningDiscordMessage,
	)

	return d.send(ctx, message)
}

func (d *discord) send(ctx context.Context, message string) error {
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
		d.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: `discord`}).Observe(duration)
	}()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("received from Discord non-204 response code: %v", resp.Status)
	}

	return nil
}

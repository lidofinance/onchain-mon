package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
)

type AlertPayload struct {
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Alias       string `json:"alias,omitempty"`
}

type opsGenie struct {
	opsGenieKey string
	httpClient  *http.Client
	metrics     *metrics.Store
	source      string
}

func NewOpsgenie(opsGenieKey string, httpClient *http.Client, metricsStore *metrics.Store, source string) *opsGenie {
	return &opsGenie{
		opsGenieKey: opsGenieKey,
		httpClient:  httpClient,
		metrics:     metricsStore,
		source:      source,
	}
}

func (u *opsGenie) SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error {
	opsGeniePriority := ""
	switch alert.Severity {
	case databus.SeverityCritical:
		opsGeniePriority = "P2"
	case databus.SeverityHigh:
		opsGeniePriority = "P3"
	}

	// Send only P2 or P3 alerts
	if opsGeniePriority == "" {
		return nil
	}

	message := FormatAlert(alert, u.source)

	payload := AlertPayload{
		Message:     alert.Name,
		Description: message,
		Alias:       alert.AlertId,
		Priority:    opsGeniePriority,
	}

	return u.send(ctx, payload)
}

func (u *opsGenie) send(ctx context.Context, payload AlertPayload) error {
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal OpsGenie payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.opsgenie.com/v2/alerts",
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("could not create OpsGenie request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "GenieKey "+u.opsGenieKey)

	start := time.Now()
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send OpsGenie request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		u.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: `opsgenie`}).Observe(duration)
	}()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received from OpsGenie non-202 response code: %v", resp.Status)
	}

	return nil
}

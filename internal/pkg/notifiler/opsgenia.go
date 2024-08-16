package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

type opsGenia struct {
	opsGenieKey string
	httpClient  *http.Client
	metrics     *metrics.Store
	source      string
}

//go:generate ./../../../bin/mockery --name OpsGenia
type OpsGenia interface {
	SendMessage(ctx context.Context, message, description, alias, priority string) error
}

func NewOpsGenia(opsGenieKey string, httpClient *http.Client, metricsStore *metrics.Store, source string) OpsGenia {
	return &opsGenia{
		opsGenieKey: opsGenieKey,
		httpClient:  httpClient,
		metrics:     metricsStore,
		source:      source,
	}
}

func (u *opsGenia) SendMessage(ctx context.Context, findingName, findingDescription, alias, priority string) error {
	type AlertPayload struct {
		Message     string `json:"message"`
		Description string `json:"description,omitempty"`
		Priority    string `json:"priority,omitempty"`
		Alias       string `json:"alias,omitempty"`
	}

	message := fmt.Sprintf("%s \nSource: %s", findingDescription, u.source)

	payload := AlertPayload{
		Message:     findingName,
		Description: message,
		Alias:       alias,
		Priority:    priority,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal OpsGenia payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.opsgenie.com/v2/alerts",
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("could not create OpsGenia request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "GenieKey "+u.opsGenieKey)

	start := time.Now()
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send OpsGenia request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		u.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: `opsgenie`}).Observe(duration)
	}()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received from OpsGenia non-202 response code: %v", resp.Status)
	}

	return nil
}

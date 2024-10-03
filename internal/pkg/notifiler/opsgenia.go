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
	"github.com/lidofinance/finding-forwarder/generated/forta/models"
	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

type AlertPayload struct {
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Alias       string `json:"alias,omitempty"`
}

type opsGenia struct {
	opsGenieKey string
	httpClient  *http.Client
	metrics     *metrics.Store
	source      string
}

func NewOpsGenia(opsGenieKey string, httpClient *http.Client, metricsStore *metrics.Store, source string) *opsGenia {
	return &opsGenia{
		opsGenieKey: opsGenieKey,
		httpClient:  httpClient,
		metrics:     metricsStore,
		source:      source,
	}
}

func (u *opsGenia) SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error {
	opsGeniaPriority := ""
	switch alert.Severity {
	case databus.SeverityCritical:
		opsGeniaPriority = "P2"
	case databus.SeverityHigh:
		opsGeniaPriority = "P3"
	}

	// Send only P2 or P3 alerts
	if opsGeniaPriority == "" {
		return nil
	}

	message := FormatAlert(alert, u.source)

	payload := AlertPayload{
		Message:     alert.Name,
		Description: message,
		Alias:       alert.AlertId,
		Priority:    opsGeniaPriority,
	}

	return u.send(ctx, payload)
}

func (u *opsGenia) SendAlert(ctx context.Context, alert *models.Alert) error {
	opsGeniaPriority := ""
	switch alert.Severity {
	case models.AlertSeverityCRITICAL:
		opsGeniaPriority = "P2"
	case models.AlertSeverityHIGH:
		opsGeniaPriority = "P3"
	}

	// Send only P2 or P3 alerts
	if opsGeniaPriority == "" {
		return nil
	}

	message := fmt.Sprintf("%s\n\nAlertId:%s\nSource: %s", alert.Description, alert.AlertID, u.source)

	payload := AlertPayload{
		Message:     alert.Name,
		Description: message,
		Alias:       alert.AlertID,
		Priority:    opsGeniaPriority,
	}

	return u.send(ctx, payload)
}

func (u *opsGenia) send(ctx context.Context, payload AlertPayload) error {
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

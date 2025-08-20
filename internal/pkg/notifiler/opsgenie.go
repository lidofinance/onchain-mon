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
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type AlertPayload struct {
	Message     string `json:"message"`
	Description string `json:"description,omitempty"`
	Priority    string `json:"priority,omitempty"`
	Alias       string `json:"alias,omitempty"`
}

type OpsGenie struct {
	opsGenieKey            string
	httpClient             *http.Client
	metrics                *metrics.Store
	blockExplorer          string
	channelID              string
	redisStreamName        string
	redisConsumerGroupName string
	source                 string
}

func NewOpsgenie(opsGenieKey string,
	httpClient *http.Client, metricsStore *metrics.Store,
	blockExplorer,
	channelID,
	redisStreamName,
	redisConsumerGroupName,
	source string,
) *OpsGenie {
	return &OpsGenie{
		opsGenieKey:            opsGenieKey,
		httpClient:             httpClient,
		metrics:                metricsStore,
		blockExplorer:          blockExplorer,
		channelID:              channelID,
		redisStreamName:        redisStreamName,
		redisConsumerGroupName: redisConsumerGroupName,
		source:                 source,
	}
}

const OpsGenieLabel = `opsgenie`
const OpsGenieRetryAfter = 5 * time.Second

func (o *OpsGenie) SendFinding(ctx context.Context, alert *databus.FindingDtoJson, quorumBy string) error {
	opsGeniePriority := ""
	switch alert.Severity {
	case databus.SeverityCritical:
		opsGeniePriority = "P1"
	case databus.SeverityHigh:
		opsGeniePriority = "P2"
	}

	// Send only P1 or P2 alerts
	if opsGeniePriority == "" {
		return nil
	}

	message := FormatAlert(alert, quorumBy, o.blockExplorer)

	payload := AlertPayload{
		Message:     alert.Name,
		Description: message,
		Alias:       alert.AlertId,
		Priority:    opsGeniePriority,
	}

	return o.send(ctx, payload)
}

func (o *OpsGenie) send(ctx context.Context, payload AlertPayload) error {
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
	req.Header.Set("Authorization", "GenieKey "+o.opsGenieKey)

	start := time.Now()
	resp, err := o.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send OpsGenie request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		o.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: OpsGenieLabel}).Observe(duration)
	}()

	if resp.StatusCode == http.StatusTooManyRequests {
		o.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: OpsGenieLabel, metrics.Status: metrics.StatusFail}).Inc()
		return &RateLimitedError{
			ResetAfter: OpsGenieRetryAfter,
			Err:        ErrRateLimited,
		}
	}

	if resp.StatusCode != http.StatusAccepted {
		o.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: OpsGenieLabel, metrics.Status: metrics.StatusFail}).Inc()
		return fmt.Errorf("received from OpsGenie non-202 response code: %v", resp.Status)
	}

	o.metrics.NotifyChannels.With(prometheus.Labels{metrics.Channel: OpsGenieLabel, metrics.Status: metrics.StatusOk}).Inc()
	return nil
}

func (o *OpsGenie) GetType() registry.NotificationChannel {
	return registry.OpsGenie
}

func (o *OpsGenie) GetChannelID() string {
	return o.channelID
}

func (o *OpsGenie) GetRedisStreamName() string {
	return fmt.Sprintf("%s:%s:%s", o.redisStreamName, o.channelID, o.source)
}
func (o *OpsGenie) GetRedisConsumerGroupName() string {
	return fmt.Sprintf("%s:%s:%s", o.redisConsumerGroupName, o.channelID, o.source)
}

package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type opsGenia struct {
	opsGenieKey string
	httpClient  *http.Client
}

//go:generate ./../../../bin/mockery --name OpsGenia
type OpsGenia interface {
	SendMessage(ctx context.Context, message, description, alias, priority string) error
}

func NewOpsGenia(opsGenieKey string, httpClient *http.Client) OpsGenia {
	return &opsGenia{
		opsGenieKey: opsGenieKey,
		httpClient:  httpClient,
	}
}

func (u *opsGenia) SendMessage(ctx context.Context, message, description, alias, priority string) error {
	type AlertPayload struct {
		Message     string `json:"message"` //
		Description string `json:"description,omitempty"`
		Priority    string `json:"priority,omitempty"`
		Alias       string `json:"alias,omitempty"`
	}

	payload := AlertPayload{
		Message:     message,
		Description: description,
		Alias:       alias,
		Priority:    priority,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST",
		"https://api.opsgenie.com/v2/alerts",
		bytes.NewBuffer(payloadBytes),
	)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "GenieKey "+u.opsGenieKey)

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("received non-202 response code: %v", resp.Status)
	}

	return nil
}

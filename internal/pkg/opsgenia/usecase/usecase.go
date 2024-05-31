package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/lidofinance/finding-forwarder/internal/pkg/opsgenia"
)

type usecase struct {
	opsGenieKey string
	httpClient  http.Client
}

func New(opsGenieKey string, httpClient http.Client) opsgenia.Usecase {
	return &usecase{
		opsGenieKey: opsGenieKey,
		httpClient:  httpClient,
	}
}

func (u *usecase) SendMessage(ctx context.Context, message, description, priority string) error {
	type AlertPayload struct {
		Message     string `json:"message"`
		Description string `json:"description,omitempty"`
		Priority    string `json:"priority,omitempty"`
	}

	payload := AlertPayload{
		Message:     message,
		Description: description,
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

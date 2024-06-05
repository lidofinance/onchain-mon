package notifiler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type discord struct {
	webhookURL string
	httpClient *http.Client
}

type Discord interface {
	SendMessage(ctx context.Context, message string) error
}

func NewDiscord(webhookURL string, httpClient *http.Client) Discord {
	return &discord{
		webhookURL: webhookURL,
		httpClient: httpClient,
	}
}

func (d *discord) SendMessage(ctx context.Context, message string) error {
	type MessagePayload struct {
		Content string `json:"content"`
	}

	payload := MessagePayload{
		Content: message,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("could not marshal payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", d.webhookURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return fmt.Errorf("error creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("received non-204 response code: %v", resp.Status)
	}

	return nil
}

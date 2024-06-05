package notifiler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
)

type telegram struct {
	botToken   string
	chatID     string
	httpClient *http.Client
}

//go:generate ./../../../bin/mockery --name Telegram
type Telegram interface {
	SendMessage(ctx context.Context, message string) error
}

func NewTelegram(botToken, chatID string, httpClient *http.Client) Telegram {
	return &telegram{
		botToken:   botToken,
		chatID:     chatID,
		httpClient: httpClient,
	}
}

func (u *telegram) SendMessage(ctx context.Context, message string) error {
	requestURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=-%s&text=%s", u.botToken, u.chatID, url.QueryEscape(message))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("could not create request: %w", err)
	}

	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %v", resp.Status)
	}

	return nil
}

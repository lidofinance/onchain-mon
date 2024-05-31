package usecase

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/lidofinance/finding-forwarder/internal/pkg/telegram"
)

type usecase struct {
	botToken   string
	httpClient http.Client
}

func New(botToken string, httpClient http.Client) telegram.Usecase {
	return &usecase{
		botToken:   botToken,
		httpClient: httpClient,
	}
}

func (u *usecase) SendMessage(ctx context.Context, chatID, message string) error {
	requestURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=-%s&text=%s", u.botToken, chatID, url.QueryEscape(message))
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

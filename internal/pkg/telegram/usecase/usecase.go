package usecase

import (
	"fmt"
	"github.com/lidofinance/finding-forwarder/internal/pkg/telegram"
	"net/http"
	"net/url"
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

func (u *usecase) SendMessage(chatID string, message string) error {
	request := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=-%s&text=%s", u.botToken, chatID, url.QueryEscape(message))

	resp, err := u.httpClient.Get(request)
	if err != nil {
		return fmt.Errorf("could not send request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received non-200 response code: %v", resp.Status)
	}

	return nil
}

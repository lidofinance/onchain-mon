package notifiler

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/finding-forwarder/internal/connectors/metrics"
)

type telegram struct {
	botToken   string
	chatID     string
	httpClient *http.Client
	metrics    *metrics.Store
}

//go:generate ./../../../bin/mockery --name Telegram
type Telegram interface {
	SendMessage(ctx context.Context, message string) error
}

func NewTelegram(botToken, chatID string, httpClient *http.Client, metricsStore *metrics.Store) Telegram {
	return &telegram{
		botToken:   botToken,
		chatID:     chatID,
		httpClient: httpClient,
		metrics:    metricsStore,
	}
}

func (u *telegram) SendMessage(ctx context.Context, message string) error {
	requestURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage?chat_id=-%s&text=%s", u.botToken, u.chatID, url.QueryEscape(message))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("could not create telegram request: %w", err)
	}

	start := time.Now()
	resp, err := u.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send telegram request: %w", err)
	}
	defer func() {
		resp.Body.Close()
		duration := time.Since(start).Seconds()
		u.metrics.SummaryHandlers.With(prometheus.Labels{metrics.Channel: `telegram`}).Observe(duration)
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("received from telegram non-200 response code: %v", resp.Status)
	}

	return nil
}

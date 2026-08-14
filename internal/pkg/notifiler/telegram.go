package notifiler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/lidofinance/onchain-mon/generated/databus"
	"github.com/lidofinance/onchain-mon/internal/connectors/metrics"
	"github.com/lidofinance/onchain-mon/internal/utils/registry"
)

type Telegram struct {
	botToken      string
	chatID        string
	httpClient    *http.Client
	metrics       *metrics.Store
	blockExplorer string
	source        string
}

type tgRes struct {
	Ok          bool   `json:"ok"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

func NewTelegram(botToken, chatID string,
	httpClient *http.Client, metricsStore *metrics.Store,
	source, blockExplorer string,
) *Telegram {
	return &Telegram{
		botToken:      botToken,
		chatID:        chatID,
		httpClient:    httpClient,
		metrics:       metricsStore,
		source:        source,
		blockExplorer: blockExplorer,
	}
}

const MaxTelegramMessageLength = 4096
const WarningTelegramMessage = "Warn: Msg >=4096, pls review description message"
const TelegramLabel = `telegram`

func (t *Telegram) SendFinding(ctx context.Context, alert *databus.FindingDtoJson) error {
	// Escape MarkdownV1 entity characters in the free-text, finding-controlled
	// fields before they are composed into the message. This is done on a copy so
	// the change is scoped to the Telegram notifier and does not affect the other
	// channels that also call FormatAlert. FormatAlert still builds its own trusted
	// inline links (block/tx) around these already-escaped values, so a crafted
	// finding can no longer smuggle links or formatting into the alert.
	safeAlert := *alert
	safeAlert.Name = escapeMarkdownV1Field(alert.Name)
	safeAlert.Description = escapeMarkdownV1Field(alert.Description)
	safeAlert.Team = escapeMarkdownV1Field(alert.Team)
	safeAlert.BotName = escapeMarkdownV1Field(alert.BotName)

	message := TruncateMessageWithAlertID(
		fmt.Sprintf("%s\n\n%s", safeAlert.Name, FormatAlert(&safeAlert, t.source, t.blockExplorer)),
		MaxTelegramMessageLength,
		WarningTelegramMessage,
	)

	if alert.Severity != databus.SeverityUnknown {
		m := escapeMarkdownV1(message)

		if sendErr := t.send(ctx, m, true); sendErr != nil {
			if errors.Is(sendErr, ErrMarkdownParse) {
				return t.send(ctx, message+"\n\nWarning: Could not send msg as markdown", false)
			}

			return sendErr
		}

		return nil
	}

	return t.send(ctx, message, false)
}

func (t *Telegram) send(ctx context.Context, message string, useMarkdown bool) error {
	requestURL := fmt.Sprintf(
		"https://api.telegram.org/bot%s/sendMessage?disable_web_page_preview=true&disable_notification=true&chat_id=-%s&text=%s",
		t.botToken,
		t.chatID,
		url.QueryEscape(message),
	)
	if useMarkdown {
		requestURL += `&parse_mode=markdown`
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, http.NoBody)
	if err != nil {
		return fmt.Errorf("could not create telegram request: %w", err)
	}

	start := time.Now()
	rawResp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("could not send telegram request: %w", err)
	}
	defer func() {
		rawResp.Body.Close()
		t.metrics.SummaryHandlers.
			With(prometheus.Labels{metrics.Channel: TelegramLabel}).
			Observe(time.Since(start).Seconds())
	}()

	var resp tgRes
	body, _ := io.ReadAll(rawResp.Body)
	_ = json.Unmarshal(body, &resp)

	if rawResp.StatusCode == http.StatusTooManyRequests || resp.ErrorCode == http.StatusTooManyRequests {
		t.metrics.NotifyChannels.
			With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusFail}).
			Inc()

		return &RateLimitedError{
			ResetAfter: time.Duration(resp.Parameters.RetryAfter) * time.Second,
			Err:        ErrRateLimited,
		}
	}

	if rawResp.StatusCode >= http.StatusBadRequest && rawResp.StatusCode < http.StatusInternalServerError {
		return fmt.Errorf("%w: %s", ErrMarkdownParse, resp.Description)
	}

	if rawResp.StatusCode != http.StatusOK || !resp.Ok {
		t.metrics.NotifyChannels.
			With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusFail}).
			Inc()

		if resp.Description != "" || resp.ErrorCode != 0 {
			return fmt.Errorf("telegram error: %s (%d)", resp.Description, resp.ErrorCode)
		}
		return fmt.Errorf("received from telegram non-200 response code: %v", rawResp.Status)
	}

	t.metrics.NotifyChannels.
		With(prometheus.Labels{metrics.Channel: TelegramLabel, metrics.Status: metrics.StatusOk}).
		Inc()
	return nil
}

func (t *Telegram) GetType() registry.NotificationChannel {
	return registry.Telegram
}

// Telegram supports two versions of markdown. V1, V2
// For V1 we have to escape some symbols
//
// V2 - is more reach for special symbols, more you can find by link
// https://core.telegram.org/bots/update56kabdkb12ibuisabdubodbasbdaosd#markdownv2-style
//
// NOTE: this escapes an already-composed message and therefore only escapes the
// underscore, because escaping the other V1 entity characters here would also
// break the intentional inline links (`[block](url)`, `[tx](url)`) that
// FormatAlert composes into the trusted footer. Escaping of the free-text,
// finding-controlled fields (Name, Description, Team, BotName) is done at their
// source in SendFinding via escapeMarkdownV1Field, so those cannot smuggle
// markup into the message.
func escapeMarkdownV1(input string) string {
	specialChars := map[string]struct{}{
		`_`: {},
	}

	var escaped strings.Builder
	for _, char := range input {
		if _, ok := specialChars[string(char)]; ok {
			escaped.WriteString(`\`)
		}

		escaped.WriteRune(char)
	}

	return escaped.String()
}

// escapeMarkdownV1Field escapes every Telegram MarkdownV1 entity character in a
// free-text field that originates from a finding (Name, Description). Unlike
// escapeMarkdownV1, which runs over the fully composed message and must preserve
// the intentional links, this runs over untrusted text before it is placed into
// the message, so it neutralises `*`, backtick and `[` in addition to `_`.
//
// Without this, a finding whose text contains e.g. `[click](https://evil)` or
// backtick-fenced content renders as active markup in the notification channel,
// letting a crafted finding inject clickable links or formatting into an
// otherwise trusted alert.
func escapeMarkdownV1Field(input string) string {
	specialChars := map[rune]struct{}{
		'_': {},
		'*': {},
		'`': {},
		'[': {},
	}

	var escaped strings.Builder
	for _, char := range input {
		if _, ok := specialChars[char]; ok {
			escaped.WriteString(`\`)
		}

		escaped.WriteRune(char)
	}

	return escaped.String()
}

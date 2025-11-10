package notifiler_test

import (
	"testing"

	"github.com/lidofinance/onchain-mon/internal/pkg/notifiler"
)

func TestAdjustMarkdownLinksToSlackWebhookFormat(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "plain text untouched",
			in:   "hello world",
			out:  "hello world",
		},
		{
			name: "markdown link converted",
			in:   "see [docs](https://example.com/docs) please",
			out:  "see <https://example.com/docs|docs> please",
		},
		{
			name: "http link converted",
			in:   "visit [site](http://example.com)",
			out:  "visit <http://example.com|site>",
		},
		{
			name: "non-http link not converted",
			in:   "open [file](/local/path)",
			out:  "open [file](/local/path)",
		},
		{
			name: "already slack-formatted link untouched",
			in:   "see <https://example.com|docs>",
			out:  "see <https://example.com|docs>",
		},
		{
			name: "bold italic strike code remain",
			in:   "*bold* _italic_ ~strike~ `code`",
			out:  "*bold* _italic_ ~strike~ `code`",
		},
		{
			name: "inline code with markdown link stays as is",
			in:   "literal: `[docs](https://example.com)` inside code: `run [cmd](https://x.y)`",
			out:  "literal: `[docs](https://example.com)` inside code: `run [cmd](https://x.y)`",
		},
		{
			name: "triple backtick code block preserved",
			in:   "before\n```go\n// [not-a-link](https://example.com)\nfmt.Println(\"hi\")\n```\nafter",
			out:  "before\n```go\n// [not-a-link](https://example.com)\nfmt.Println(\"hi\")\n```\nafter",
		},
		{
			name: "multiple links converted",
			in:   "A [one](https://a.com) and [two](https://b.com).",
			out:  "A <https://a.com|one> and <https://b.com|two>.",
		},
		{
			name: "broken markdown no change",
			in:   "[broken link](https://ok\nstill text",
			out:  "[broken link](https://ok\nstill text",
		},
		{
			name: "brackets without url part untouched",
			in:   "[label] and text",
			out:  "[label] and text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := notifiler.AdjustMarkdownLinksToSlackWebhookFormat(tt.in)
			if got != tt.out {
				t.Fatalf("got:\n%q\nwant:\n%q", got, tt.out)
			}
		})
	}
}

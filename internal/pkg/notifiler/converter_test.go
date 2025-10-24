package notifiler

import "testing"

func TestConvertDiscordToSlack(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty string",
			in:   "",
			want: "",
		},
		{
			name: "plain text",
			in:   "hello world",
			want: "hello world",
		},

		// Links
		{
			name: "simple link",
			in:   "[foo](https://example.com)",
			want: "<https://example.com|foo>",
		},
		{
			name: "multiple links",
			in:   "[a](https://a.example) and [b](http://b.example)",
			want: "<https://a.example|a> and <http://b.example|b>",
		},
		{
			name: "non-http link ignored",
			in:   "[text](ftp://server.com)",
			want: "[text](ftp://server.com)",
		},
		{
			name: "incomplete link",
			in:   "[text](incomplete",
			want: "[text](incomplete",
		},
		{
			name: "link with newline ignored",
			in:   "[text\n](https://x)",
			want: "[text\n](https://x)",
		},

		// Bold
		{
			name: "bold",
			in:   "**bold**",
			want: "*bold*",
		},
		{
			name: "bold in sentence",
			in:   "text **bold** text",
			want: "text *bold* text",
		},
		{
			name: "unclosed bold",
			in:   "**bold without closing",
			want: "*bold without closing",
		},

		// Italic
		{
			name: "italic",
			in:   "a *i* b",
			want: "a _i_ b",
		},
		{
			name: "italic at start",
			in:   "*italic* text",
			want: "_italic_ text",
		},
		{
			name: "italic at end",
			in:   "text *italic*",
			want: "text _italic_",
		},
		{
			name: "italic after punctuation",
			in:   "( *paren* )",
			want: "( _paren_ )",
		},
		{
			name: "italic before punctuation",
			in:   "*word*, next",
			want: "_word_, next",
		},
		{
			name: "asterisk inside word not italic",
			in:   "pre*fix",
			want: "pre*fix",
		},
		{
			name: "unclosed italic",
			in:   "*italic without closing",
			want: "_italic without closing",
		},

		// Strike
		{
			name: "strikethrough",
			in:   "~~strike~~",
			want: "~strike~",
		},
		{
			name: "strikethrough in text",
			in:   "text ~~del~~ text",
			want: "text ~del~ text",
		},

		// Code blocks
		{
			name: "code block masks markdown",
			in:   "Start\n```go\n**bold** *italic* [l](https://a)\n```\nEnd",
			want: "Start\n```go\n**bold** *italic* [l](https://a)\n```\nEnd",
		},
		{
			name: "plain code block",
			in:   "X\n```\n**b**\n```\nY",
			want: "X\n```\n**b**\n```\nY",
		},
		{
			name: "unclosed code block",
			in:   "```\n**text**",
			want: "```\n**text**",
		},

		// Inline code
		{
			name: "inline code masks markdown",
			in:   "text `**bold** *italic*` text",
			want: "text `**bold** *italic*` text",
		},
		{
			name: "bold around inline code",
			in:   "**bold `code`**",
			want: "*bold `code`*",
		},
		{
			name: "unclosed inline code",
			in:   "text `code without closing",
			want: "text `code without closing",
		},

		// Escaping
		{
			name: "escaped asterisk",
			in:   "\\*not italic\\*",
			want: "*not italic*",
		},
		{
			name: "escaped double asterisk",
			in:   "\\**not bold\\**",
			want: "**not bold**",
		},
		{
			name: "escaped backtick",
			in:   "\\`not code\\`",
			want: "`not code`",
		},
		{
			name: "escaped tilde",
			in:   "\\~~not strike\\~~",
			want: "~~not strike~~",
		},
		{
			name: "escaped brackets",
			in:   "\\[not\\](link)",
			want: "[not](link)",
		},
		{
			name: "escape in middle of bold",
			in:   "**bold \\* text**",
			want: "*bold * text*",
		},
		{
			name: "multiple escapes",
			in:   "\\*\\*text\\*\\*",
			want: "**text**",
		},
		{
			name: "backslash before non-markdown",
			in:   "\\a \\1",
			want: "\\a \\1",
		},

		// Combined formatting
		{
			name: "all formats",
			in:   "**bold** and *italic* with [link](https://x) and ~~strike~~",
			want: "*bold* and _italic_ with <https://x|link> and ~strike~",
		},
		{
			name: "nested formats",
			in:   "**bold with *italic* inside**",
			want: "*bold with _italic_ inside*",
		},
		{
			name: "link with bold text",
			in:   "[**bold link**](https://example.com)",
			want: "<https://example.com|**bold link**>",
		},
		{
			name: "code block then formats",
			in:   "```\ncode\n```\n**bold** *italic*",
			want: "```\ncode\n```\n*bold* _italic_",
		},

		// Edge cases
		{
			name: "triple asterisk",
			in:   "***text***",
			want: "**text**",
		},
		{
			name: "multiple spaces",
			in:   "**bold**  *italic*",
			want: "*bold*  _italic_",
		},
		{
			name: "newlines preserved",
			in:   "line1\n**bold**\nline3",
			want: "line1\n*bold*\nline3",
		},
		{
			name: "unicode text",
			in:   "**脂肪の多い** *イタリック体* こんにちは",
			want: "*脂肪の多い* _イタリック体_ こんにちは",
		},
		{
			name: "empty bold",
			in:   "****",
			want: "**",
		},
		{
			name: "empty italic",
			in:   "**",
			want: "*",
		},
		{
			name: "asterisk alone",
			in:   "text * text",
			want: "text * text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeMarkdownForSlack(tt.in)
			if got != tt.want {
				t.Errorf("NormalizeMarkdownForSlack()\ngot:  %q\nwant: %q", got, tt.want)
			}
		})
	}
}

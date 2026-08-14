package notifiler

import "testing"

func Test_escapeMarkdownV1Field(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain text is unchanged",
			input: "Vault balance dropped",
			want:  "Vault balance dropped",
		},
		{
			name:  "underscore is escaped",
			input: "low_balance_alert",
			want:  `low\_balance\_alert`,
		},
		{
			name:  "injected inline link is neutralised",
			input: "[click me](https://evil.example)",
			want:  `\[click me](https://evil.example)`,
		},
		{
			name:  "asterisk emphasis is neutralised",
			input: "*urgent*",
			want:  `\*urgent\*`,
		},
		{
			name:  "backtick code span is neutralised",
			input: "run `rm -rf` now",
			want:  "run \\`rm -rf\\` now",
		},
		{
			name:  "all entity characters together",
			input: "_*`[",
			want:  "\\_\\*\\`\\[",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := escapeMarkdownV1Field(tt.input); got != tt.want {
				t.Errorf("escapeMarkdownV1Field(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

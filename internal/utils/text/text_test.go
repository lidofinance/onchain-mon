package text

import "testing"

func TestLeaveOnlyDomainInURLs(t *testing.T) {
	type args struct {
		input string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "leave only domain",
			args: args{
				input: "GET https://api.example.com/v1/users?id=123&token=foobar",
			},
			want: "GET https://api.example.com",
		},
		{
			name: "leave only domain url",
			args: args{
				input: "FetchBlockByNumber error: All attempts fail:\\n#1: could not send request: " +
					"Post \\\"https://api.example.com/foobar\\\": context deadline exceeded " +
					"(Client.Timeout exceeded while awaiting headers)",
			},
			want: "FetchBlockByNumber error: All attempts fail:\\n#1: could not send request: " +
				"Post \\\"https://api.example.com\": context deadline exceeded " +
				"(Client.Timeout exceeded while awaiting headers)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := LeaveOnlyDomainInURLs(tt.args.input); got != tt.want {
				t.Errorf("LeaveOnlyDomainInURLs() = %v, want %v", got, tt.want)
			}
		})
	}
}

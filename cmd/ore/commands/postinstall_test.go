package commands

import "testing"

func TestNormalizeRubyLiteral(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "quoted freeze", input: `"google-api-client".freeze`, want: "google-api-client"},
		{name: "version freeze", input: `"0.53.0".freeze`, want: "0.53.0"},
		{name: "parenthesized", input: `("httparty").freeze`, want: "httparty"},
		{name: "multiline escapes", input: `"line1\nline2".freeze`, want: "line1\nline2"},
		{name: "percent string", input: "%Q{hello\\nworld}", want: "hello\nworld"},
		{name: "percent string single quote", input: "%q{hello\\nworld}", want: "hello\\nworld"},
		{
			name:  "unicode escapes",
			input: "\"\\u26A0\\uFE0F\".freeze",
			want:  string([]rune{0x26A0, 0xFE0F}),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := normalizeRubyLiteral(tt.input); got != tt.want {
				t.Fatalf("normalizeRubyLiteral(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

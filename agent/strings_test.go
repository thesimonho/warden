package agent

import "testing"

func TestTruncateString(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		input  string
		maxLen int
		want   string
	}{
		{name: "short string", input: "hello", maxLen: 10, want: "hello"},
		{name: "exact length", input: "hello", maxLen: 5, want: "hello"},
		{name: "truncated", input: "hello world", maxLen: 5, want: "hello…"},
		{name: "empty", input: "", maxLen: 5, want: ""},
		{name: "unicode safe", input: "日本語テスト", maxLen: 3, want: "日本語…"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateString(tc.input, tc.maxLen)
			if got != tc.want {
				t.Errorf("TruncateString(%q, %d) = %q, want %q", tc.input, tc.maxLen, got, tc.want)
			}
		})
	}
}

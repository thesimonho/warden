package agent

// TruncateString caps a string at maxLen runes, appending "…" if truncated.
// Uses rune count to avoid splitting multi-byte UTF-8 characters.
func TruncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

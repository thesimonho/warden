package tui

import "fmt"

// padRight pads a string with spaces to the given width.
func padRight(s string, width int) string {
	return fmt.Sprintf("%-*s", width, s)
}

// truncate shortens a string to maxLen, adding an ellipsis if truncated.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-1] + "…"
}

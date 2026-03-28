package components

import "fmt"

// FormatCost formats a dollar amount for display (e.g. "$0.42").
// Port of web/src/lib/cost.ts formatCost.
func FormatCost(dollars float64) string {
	return fmt.Sprintf("$%.2f", dollars)
}

// FormatBudgetProgress formats cost with an optional budget limit.
// Returns "$1.23/$5.00" when a budget is set, or "$1.23" otherwise.
func FormatBudgetProgress(cost, budget float64) string {
	if budget > 0 {
		return fmt.Sprintf("$%.2f/$%.2f", cost, budget)
	}
	return FormatCost(cost)
}

// FormatDuration formats milliseconds to a human-readable string.
// Examples: "2m 30s", "1h 15m", "0s".
// Port of web/src/lib/cost.ts formatDuration.
func FormatDuration(ms int) string {
	totalSeconds := ms / 1000
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		if minutes > 0 {
			return fmt.Sprintf("%dh %dm", hours, minutes)
		}
		return fmt.Sprintf("%dh", hours)
	}
	if minutes > 0 {
		if seconds > 0 {
			return fmt.Sprintf("%dm %ds", minutes, seconds)
		}
		return fmt.Sprintf("%dm", minutes)
	}
	return fmt.Sprintf("%ds", seconds)
}

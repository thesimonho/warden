package components

import "testing"

func TestFormatCost(t *testing.T) {
	t.Parallel()

	tests := []struct {
		dollars float64
		want    string
	}{
		{0, "$0.00"},
		{0.42, "$0.42"},
		{1.5, "$1.50"},
		{10.99, "$10.99"},
		{100.123, "$100.12"},
		{0.001, "$0.00"},
	}
	for _, tt := range tests {
		got := FormatCost(tt.dollars)
		if got != tt.want {
			t.Errorf("FormatCost(%v) = %q, want %q", tt.dollars, got, tt.want)
		}
	}
}

func TestFormatBudgetProgress(t *testing.T) {
	t.Parallel()

	tests := []struct {
		cost, budget float64
		want         string
	}{
		{1.23, 0, "$1.23"},
		{1.23, 5.00, "$1.23/$5.00"},
		{0, 10, "$0.00/$10.00"},
		{5.50, 5.00, "$5.50/$5.00"},
	}
	for _, tt := range tests {
		got := FormatBudgetProgress(tt.cost, tt.budget)
		if got != tt.want {
			t.Errorf("FormatBudgetProgress(%v, %v) = %q, want %q", tt.cost, tt.budget, got, tt.want)
		}
	}
}

func TestFormatDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		ms   int
		want string
	}{
		{0, "0s"},
		{500, "0s"},
		{1000, "1s"},
		{30_000, "30s"},
		{60_000, "1m"},
		{90_000, "1m 30s"},
		{150_000, "2m 30s"},
		{3_600_000, "1h"},
		{4_500_000, "1h 15m"},
		{3_660_000, "1h 1m"},
		{7_200_000, "2h"},
	}
	for _, tt := range tests {
		got := FormatDuration(tt.ms)
		if got != tt.want {
			t.Errorf("FormatDuration(%d) = %q, want %q", tt.ms, got, tt.want)
		}
	}
}

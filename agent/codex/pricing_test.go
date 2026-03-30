package codex

import (
	"math"
	"testing"

	"github.com/thesimonho/warden/agent"
)

func TestEstimateCost_KnownModel(t *testing.T) {
	t.Parallel()

	tokens := agent.TokenUsage{
		InputTokens:     10000,
		OutputTokens:    500,
		CacheReadTokens: 8000,
	}

	cost := EstimateCost("gpt-5.4", tokens)

	// Uncached input: 10000-8000 = 2000 * 2.5/1e6 = 0.005
	// Cached input: 8000 * 0.25/1e6 = 0.002
	// Output: 500 * 15/1e6 = 0.0075
	// Total = 0.0145
	expected := 0.0145
	if math.Abs(cost-expected) > 0.0001 {
		t.Errorf("EstimateCost = %f, want ~%f", cost, expected)
	}
}

func TestEstimateCost_UnknownModel(t *testing.T) {
	t.Parallel()

	tokens := agent.TokenUsage{InputTokens: 1000, OutputTokens: 500}
	cost := EstimateCost("unknown-model", tokens)

	if cost != 0 {
		t.Errorf("EstimateCost for unknown model = %f, want 0", cost)
	}
}

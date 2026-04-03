package claudecode

import (
	"math"
	"testing"

	"github.com/thesimonho/warden/agent"
)

func TestEstimateCost_KnownModel(t *testing.T) {
	t.Parallel()

	tokens := agent.TokenUsage{
		InputTokens:      1000,
		OutputTokens:     500,
		CacheWriteTokens: 2000,
		CacheReadTokens:  3000,
	}

	cost := EstimateCost("claude-sonnet-4-6", tokens)

	// Expected: 1000*3/1e6 + 500*15/1e6 + 2000*3.75/1e6 + 3000*0.3/1e6
	// = 0.003 + 0.0075 + 0.0075 + 0.0009 = 0.0189
	expected := 0.0189
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

func TestEstimateCost_ZeroTokens(t *testing.T) {
	t.Parallel()

	cost := EstimateCost("claude-sonnet-4-6", agent.TokenUsage{})
	if cost != 0 {
		t.Errorf("EstimateCost for zero tokens = %f, want 0", cost)
	}
}

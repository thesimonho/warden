package claudecode

import "github.com/thesimonho/warden/agent"

// modelPricing maps Claude model IDs to per-token prices in USD.
// Prices are per million tokens, divided here to per-token.
type modelPricing struct {
	inputPerToken       float64
	outputPerToken      float64
	cacheWritePerToken  float64
	cacheReadPerToken   float64
}

// pricingTable maps known Claude model IDs to their token pricing.
// Source: https://docs.anthropic.com/en/docs/about-claude/pricing
// Updated: 2026-03 — verify periodically.
var pricingTable = map[string]modelPricing{
	"claude-opus-4-6":            {inputPerToken: 15.0 / 1e6, outputPerToken: 75.0 / 1e6, cacheWritePerToken: 18.75 / 1e6, cacheReadPerToken: 1.5 / 1e6},
	"claude-sonnet-4-6":          {inputPerToken: 3.0 / 1e6, outputPerToken: 15.0 / 1e6, cacheWritePerToken: 3.75 / 1e6, cacheReadPerToken: 0.3 / 1e6},
	"claude-haiku-4-5-20251001":  {inputPerToken: 0.8 / 1e6, outputPerToken: 4.0 / 1e6, cacheWritePerToken: 1.0 / 1e6, cacheReadPerToken: 0.08 / 1e6},
	"claude-sonnet-4-5-20250514": {inputPerToken: 3.0 / 1e6, outputPerToken: 15.0 / 1e6, cacheWritePerToken: 3.75 / 1e6, cacheReadPerToken: 0.3 / 1e6},
	"claude-opus-4-5-20250918":   {inputPerToken: 15.0 / 1e6, outputPerToken: 75.0 / 1e6, cacheWritePerToken: 18.75 / 1e6, cacheReadPerToken: 1.5 / 1e6},
}

// EstimateCost computes an estimated cost in USD from cumulative token usage.
// This is a fallback — the preferred cost source is actual cost from
// .claude.json via StatusProvider. Returns 0 for unknown models.
func EstimateCost(model string, tokens agent.TokenUsage) float64 {
	pricing, ok := pricingTable[model]
	if !ok {
		return 0
	}

	cost := float64(tokens.InputTokens) * pricing.inputPerToken
	cost += float64(tokens.OutputTokens) * pricing.outputPerToken
	cost += float64(tokens.CacheWriteTokens) * pricing.cacheWritePerToken
	cost += float64(tokens.CacheReadTokens) * pricing.cacheReadPerToken

	return cost
}

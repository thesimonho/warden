package codex

import "github.com/thesimonho/warden/agent"

// modelPricing maps model IDs to per-token prices in USD.
type modelPricing struct {
	inputPerToken       float64
	cachedInputPerToken float64
	outputPerToken      float64
}

// pricingTable maps known OpenAI model IDs to their token pricing.
// Source: https://developers.openai.com/api/docs/pricing
// Updated: 2026-03 — verify periodically.
var pricingTable = map[string]modelPricing{
	"gpt-5.4": {
		inputPerToken:       2.5 / 1e6,
		cachedInputPerToken: 0.25 / 1e6,
		outputPerToken:      15.0 / 1e6,
	},
	"gpt-5.4-mini": {
		inputPerToken:       0.75 / 1e6,
		cachedInputPerToken: 0.075 / 1e6,
		outputPerToken:      4.5 / 1e6,
	},
	"gpt-5.3-codex": {
		inputPerToken:       1.75 / 1e6,
		cachedInputPerToken: 0.175 / 1e6,
		outputPerToken:      14.0 / 1e6,
	},
}

// EstimateCost computes an estimated cost in USD from cumulative token usage.
// Codex has no actual-cost source, so this is always the primary cost.
// Returns 0 for unknown models.
func EstimateCost(model string, tokens agent.TokenUsage) float64 {
	pricing, ok := pricingTable[model]
	if !ok {
		return 0
	}

	// Cached tokens are cheaper — separate them from uncached input.
	uncachedInput := tokens.InputTokens - tokens.CacheReadTokens
	if uncachedInput < 0 {
		uncachedInput = 0
	}

	cost := float64(uncachedInput) * pricing.inputPerToken
	cost += float64(tokens.CacheReadTokens) * pricing.cachedInputPerToken
	cost += float64(tokens.OutputTokens) * pricing.outputPerToken

	return cost
}

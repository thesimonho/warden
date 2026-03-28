// Package agent defines interfaces and types for extracting status data
// from CLI agents running inside project containers. The abstraction allows
// supporting different agent backends (Claude Code, Aider, etc.) without
// coupling the dashboard to any single agent's data format.
package agent

// Status holds agent-reported data for a single session/project directory.
// Fields are optional — not all agents report all fields.
type Status struct {
	// CostUSD is the total session cost in US dollars.
	CostUSD float64

	// DurationMs is the total wall-clock time since the session started.
	DurationMs int64

	// APIDurationMs is the time spent waiting for API responses.
	APIDurationMs int64

	// LinesAdded is the total number of lines added during the session.
	LinesAdded int

	// LinesRemoved is the total number of lines removed during the session.
	LinesRemoved int

	// Model holds the agent model information.
	Model ModelInfo

	// Tokens holds token usage counters.
	Tokens TokenUsage

	// AgentSessionID is the agent's own session identifier (not the dashboard's).
	AgentSessionID string
}

// ModelInfo identifies the model being used by the agent.
type ModelInfo struct {
	ID          string
	DisplayName string
}

// TokenUsage holds token consumption counters.
type TokenUsage struct {
	InputTokens      int64
	OutputTokens     int64
	CacheReadTokens  int64
	CacheWriteTokens int64
}

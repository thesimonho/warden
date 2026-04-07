// Package agent defines interfaces and types for extracting status data
// from CLI agents running inside project containers. The abstraction allows
// supporting different agent backends (Claude Code, Aider, etc.) without
// coupling the dashboard to any single agent's data format.
package agent

import "github.com/thesimonho/warden/event"

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

// ParsedEventType is an alias for [event.ContainerEventType]. Parsers emit
// the same type constants used by the event pipeline, eliminating the need
// for a translation layer. The alias preserves readability in parser code.
type ParsedEventType = event.ContainerEventType

// ParsedEvent is an agent-agnostic event produced by parsing a session JSONL line.
// The parser converts agent-specific JSONL formats into these uniform events,
// which are then converted to [event.ContainerEvent] for SSE broadcast and audit logging.
type ParsedEvent struct {
	// Type identifies what kind of event this is.
	Type ParsedEventType

	// SessionID is the agent's session identifier.
	SessionID string
	// Timestamp is when the event occurred (ISO 8601).
	Timestamp string

	// Model is the AI model used (populated on assistant events).
	Model string
	// ToolName is the tool invoked (populated on ToolUse events).
	ToolName string
	// ToolInput is a summary of the tool input (populated on ToolUse events, truncated).
	ToolInput string
	// Prompt is the user's message text (populated on UserPrompt events).
	Prompt string
	// PromptSource classifies the origin of the prompt (populated on UserPrompt events).
	// Values: "user" (normal text), "bash" (! command), "bash_output" (! stdout/stderr).
	PromptSource PromptSource
	// ErrorContent is the error message (populated on ToolUseFailure and StopFailure events).
	ErrorContent string
	// ServerName is the MCP server name (populated on Elicitation events).
	ServerName string

	// DurationMs is the turn duration in milliseconds (populated on TurnDuration events).
	DurationMs int64

	// Tokens holds cumulative token usage (populated on TokenUpdate events).
	Tokens TokenUsage
	// EstimatedCostUSD is the estimated cost from tokens (populated on TokenUpdate events).
	EstimatedCostUSD float64

	// GitBranch is the current git branch (populated on SessionStart).
	GitBranch string
	// WorktreeID is the worktree identifier (populated on SessionStart if available).
	WorktreeID string

	// Subtype is the system message subtype (populated on SystemInfo events).
	Subtype string
	// Content is the message text (populated on SystemInfo, SubagentStop, PermissionGrant, ContextCompact events).
	Content string
	// Commands holds allowed commands (populated on PermissionGrant events).
	Commands []string
	// TTFTMs is time to first token in milliseconds (populated on ApiMetrics events).
	TTFTMs float64
	// OutputTokensPerSec is output tokens per second (populated on ApiMetrics events).
	OutputTokensPerSec float64
	// CompactTrigger is what triggered context compaction (populated on ContextCompact events).
	CompactTrigger string
	// PreCompactTokens is the token count before compaction (populated on ContextCompact events).
	PreCompactTokens int64

	// SourceLine is the raw JSONL line bytes that produced this event.
	// Used to compute a content hash for deduplication in the audit DB.
	// Set by the session watcher; empty for hook-sourced events.
	SourceLine []byte
	// SourceIndex disambiguates multiple events parsed from the same JSONL line.
	SourceIndex int
}

// ProjectInfo provides project metadata for session file discovery.
type ProjectInfo struct {
	// ProjectID is the deterministic 12-char hex project identifier.
	ProjectID string
	// AgentType identifies the agent (e.g. "claude-code", "codex").
	AgentType string
	// WorkspaceDir is the container-side workspace directory.
	WorkspaceDir string
	// ProjectName is the user-chosen project name.
	ProjectName string
}

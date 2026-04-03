package codex

import "encoding/json"

// JSONL session file types for OpenAI Codex CLI. These structs match the
// JSONL format written by Codex at ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
//
// Only fields needed for event parsing are included. Unknown fields are
// silently ignored by json.Unmarshal, providing forward compatibility.

// RolloutItem is the top-level structure of every JSONL line.
// The Type field determines which payload structure is used.
type RolloutItem struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

// SessionMeta is the payload for "session_meta" entries.
// Contains session identity, working directory, and git info.
type SessionMeta struct {
	ID            string   `json:"id"`
	Timestamp     string   `json:"timestamp"`
	CWD           string   `json:"cwd"`
	CLIVersion    string   `json:"cli_version"`
	ModelProvider string   `json:"model_provider"`
	Git           *GitInfo `json:"git,omitempty"`
}

// GitInfo holds git repository data from session metadata.
type GitInfo struct {
	CommitHash    string `json:"commit_hash"`
	Branch        string `json:"branch"`
	RepositoryURL string `json:"repository_url"`
}

// TurnContext is the payload for "turn_context" entries.
// Contains model, approval policy, and sandbox configuration per turn.
type TurnContext struct {
	TurnID         string `json:"turn_id"`
	CWD            string `json:"cwd"`
	Model          string `json:"model"`
	ApprovalPolicy string `json:"approval_policy"`
}

// ResponseItem is the payload for "response_item" entries.
// The Type field distinguishes function calls, messages, and reasoning.
type ResponseItem struct {
	Type      string          `json:"type"`
	Role      string          `json:"role,omitempty"`
	Name      string          `json:"name,omitempty"`
	CallID    string          `json:"call_id,omitempty"`
	Arguments string          `json:"arguments,omitempty"`
	Output    string          `json:"output,omitempty"`
	Status    string          `json:"status,omitempty"`
	Content   json.RawMessage `json:"content,omitempty"`

	// Action holds nested data for local_shell_call and web_search_call items.
	Action *ActionPayload `json:"action,omitempty"`

	// CustomToolCall fields (type == "custom_tool_call").
	Input string `json:"input,omitempty"`
}

// ActionPayload is the nested action payload for local_shell_call and
// web_search_call items. Both use the "action" JSON key with different fields.
type ActionPayload struct {
	Command []string `json:"command,omitempty"` // local_shell_call
	Query   string   `json:"query,omitempty"`   // web_search_call
}

// CompactedItem is the payload for top-level "compacted" entries.
type CompactedItem struct {
	Message string `json:"message,omitempty"`
}

// EventMsg is the payload for "event_msg" entries.
// The Type field distinguishes token counts, user messages, task lifecycle, etc.
type EventMsg struct {
	Type    string `json:"type"`
	TurnID  string `json:"turn_id,omitempty"`
	Message string `json:"message,omitempty"`
	Reason  string `json:"reason,omitempty"`

	// Token count fields (type == "token_count").
	Info       *TokenCountInfo `json:"info,omitempty"`
	RateLimits *RateLimits     `json:"rate_limits,omitempty"`

	// Command approval fields (app-server only — never persisted in limited mode).
	Command    string `json:"command,omitempty"`
	ServerName string `json:"server_name,omitempty"`

	// Exec command fields (type == "exec_command_end", extended mode only).
	CallID   string `json:"call_id,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
	Status   string `json:"status,omitempty"`

	// MCP tool call fields (type == "mcp_tool_call_end", extended mode only).
	ToolName      string `json:"tool_name,omitempty"`
	McpServerName string `json:"mcp_server_name,omitempty"`

	// Patch apply fields (type == "patch_apply_end", extended mode only).
	FilePath string `json:"file_path,omitempty"`

	// ThreadRolledBack fields (type == "thread_rolled_back").
	NumTurns int `json:"num_turns,omitempty"`
}

// TokenCountInfo holds cumulative and per-turn token usage.
type TokenCountInfo struct {
	TotalTokenUsage TokenUsageDetail `json:"total_token_usage"`
	LastTokenUsage  TokenUsageDetail `json:"last_token_usage"`
}

// TokenUsageDetail holds token consumption counters for Codex.
type TokenUsageDetail struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
	TotalTokens           int64 `json:"total_tokens"`
}

// RateLimits holds rate limit and subscription information.
type RateLimits struct {
	LimitID  string   `json:"limit_id"`
	Credits  *Credits `json:"credits,omitempty"`
	PlanType string   `json:"plan_type"`
}

// Credits holds subscription credit information.
type Credits struct {
	HasCredits bool    `json:"has_credits"`
	Unlimited  bool    `json:"unlimited"`
	Balance    float64 `json:"balance"`
}

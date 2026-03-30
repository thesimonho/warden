package claudecode

// JSONL session file types for Claude Code. These structs match the JSONL
// format written by Claude Code at ~/.claude/projects/<path>/<session>.jsonl.
//
// Only fields needed for event parsing are included. Unknown fields are
// silently ignored by json.Unmarshal, providing forward compatibility
// when Claude Code adds new fields.

// SessionEntry is the top-level structure of every JSONL line.
// The Type field determines which other fields are populated.
type SessionEntry struct {
	// Common fields present on all entry types.
	Type      string `json:"type"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	SessionID string `json:"sessionId"`
	CWD       string `json:"cwd"`
	GitBranch string `json:"gitBranch"`

	// Message is populated for "assistant" and "user" type entries.
	Message *MessageBody `json:"message,omitempty"`

	// Subtype distinguishes system entry kinds (e.g. "turn_duration", "informational").
	Subtype string `json:"subtype,omitempty"`
	// DurationMs is the turn duration (system entries with subtype "turn_duration").
	DurationMs int64 `json:"durationMs,omitempty"`
	// Content is the text content for system entries.
	Content string `json:"content,omitempty"`
	// Level is the severity for system entries (e.g. "warning").
	Level string `json:"level,omitempty"`
}

// MessageBody holds the message payload for assistant and user entries.
type MessageBody struct {
	Role       string       `json:"role"`
	Model      string       `json:"model,omitempty"`
	StopReason string       `json:"stop_reason,omitempty"`
	Usage      *UsageInfo   `json:"usage,omitempty"`
	Content    ContentField `json:"content"`
}

// ContentField handles the polymorphic content field — it can be either
// a plain string (user text prompts) or an array of ContentBlock objects
// (assistant responses, tool results).
type ContentField struct {
	// Text is populated when content is a plain string.
	Text string
	// Blocks is populated when content is an array of content blocks.
	Blocks []ContentBlock
}

// ContentBlock represents a single block within a message's content array.
// The Type field determines which other fields are populated.
type ContentBlock struct {
	Type string `json:"type"`
	// Text is the content for "text" type blocks.
	Text string `json:"text,omitempty"`
	// Name is the tool name for "tool_use" type blocks.
	Name string `json:"name,omitempty"`
	// Input is the tool input for "tool_use" type blocks.
	Input map[string]any `json:"input,omitempty"`
	// ToolUseID is the tool use identifier for "tool_result" type blocks.
	ToolUseID string `json:"tool_use_id,omitempty"`
}

// UsageInfo holds token consumption data from an assistant message.
type UsageInfo struct {
	InputTokens                int64 `json:"input_tokens"`
	OutputTokens               int64 `json:"output_tokens"`
	CacheCreationInputTokens   int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens       int64 `json:"cache_read_input_tokens"`
}

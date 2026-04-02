package claudecode

import "encoding/json"

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

	// TTFTMs is time to first token in ms (system/api_metrics).
	TTFTMs float64 `json:"ttftMs,omitempty"`
	// OutputTokensPerSec is output tokens per second (system/api_metrics).
	OutputTokensPerSec float64 `json:"otps,omitempty"`
	// Commands is a list of allowed commands (system/permission_retry).
	Commands []string `json:"commands,omitempty"`
	// CompactMetadata holds context compaction details (system/compact_boundary).
	CompactMetadata *CompactMetadata `json:"compactMetadata,omitempty"`

	// Operation distinguishes queue-operation entry kinds ("enqueue", "remove").
	Operation string `json:"operation,omitempty"`
}

// CompactMetadata holds details about a context compaction event.
type CompactMetadata struct {
	Trigger            string `json:"trigger"`
	PreTokens          int64  `json:"preTokens"`
	MessagesSummarized int    `json:"messagesSummarized,omitempty"`
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
	// ID is the tool use identifier for "tool_use" type blocks.
	ID string `json:"id,omitempty"`
	// Input is the tool input for "tool_use" type blocks.
	Input map[string]any `json:"input,omitempty"`
	// ToolUseID is the tool use identifier for "tool_result" type blocks.
	ToolUseID string `json:"tool_use_id,omitempty"`
	// IsError is true when a tool_result block represents an error.
	IsError bool `json:"is_error,omitempty"`
	// Content is the text content for "tool_result" type blocks.
	// Can be a string or an array of blocks — we only use the string form.
	Content json.RawMessage `json:"content,omitempty"`
}

// ErrorContent returns the text content of a tool_result error block.
// Handles both plain string content and array-of-blocks content.
func (b *ContentBlock) ErrorContent() string {
	if len(b.Content) == 0 {
		return ""
	}
	// Try plain string first.
	var s string
	if err := json.Unmarshal(b.Content, &s); err == nil {
		return s
	}
	// Try array of text blocks.
	var blocks []struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(b.Content, &blocks); err == nil && len(blocks) > 0 {
		return blocks[0].Text
	}
	return string(b.Content)
}

// UsageInfo holds token consumption data from an assistant message.
type UsageInfo struct {
	InputTokens              int64 `json:"input_tokens"`
	OutputTokens             int64 `json:"output_tokens"`
	CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
}

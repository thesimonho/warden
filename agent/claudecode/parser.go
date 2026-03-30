package claudecode

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/thesimonho/warden/agent"
)

// maxToolInputLength is the maximum length of tool input included in events.
const maxToolInputLength = 1000

// Parser implements agent.SessionParser for Claude Code session JSONL files.
// It is stateful — token counts accumulate across lines within a session.
// Create a new Parser for each session file.
type Parser struct {
	// cumulativeTokens tracks running totals across the session.
	cumulativeTokens agent.TokenUsage
	// lastModel tracks the most recently seen model name.
	lastModel string
}

// NewParser creates a new Claude Code JSONL parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseLine parses a single JSONL line into zero or more ParsedEvents.
// Returns nil for entry types that don't map to Warden events.
func (p *Parser) ParseLine(line []byte) []agent.ParsedEvent {
	var entry SessionEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	switch entry.Type {
	case "assistant":
		return p.parseAssistant(entry)
	case "user":
		return p.parseUser(entry)
	case "system":
		return p.parseSystem(entry)
	default:
		// file-history-snapshot, queue-operation, last-prompt — no events.
		return nil
	}
}

// SessionDir returns the host-side directory containing Claude Code session
// JSONL files for a project. Claude encodes the container-side workspace
// path by replacing "/" with "-" to form the directory name.
func (p *Parser) SessionDir(homeDir string, project agent.ProjectInfo) string {
	// Claude stores sessions at ~/.claude/projects/<encoded-path>/
	// where <encoded-path> replaces "/" with "-" in the container workspace path.
	encoded := encodeWorkspacePath(project.WorkspaceDir)
	return fmt.Sprintf("%s/.claude/projects/%s", homeDir, encoded)
}

// encodeWorkspacePath converts a container workspace path to Claude's
// directory encoding: replace "/" with "-".
// Example: "/home/dev/warden" → "-home-dev-warden"
func encodeWorkspacePath(workspaceDir string) string {
	return strings.ReplaceAll(workspaceDir, "/", "-")
}

// parseAssistant handles assistant-type JSONL entries. Produces ToolUse events
// for tool_use content blocks and TokenUpdate events when usage data is present.
// Also produces TurnComplete when stop_reason is "end_turn".
func (p *Parser) parseAssistant(entry SessionEntry) []agent.ParsedEvent {
	if entry.Message == nil {
		return nil
	}

	var events []agent.ParsedEvent
	msg := entry.Message

	if msg.Model != "" {
		p.lastModel = msg.Model
	}

	// Accumulate token usage.
	if msg.Usage != nil {
		p.cumulativeTokens.InputTokens += msg.Usage.InputTokens
		p.cumulativeTokens.OutputTokens += msg.Usage.OutputTokens
		p.cumulativeTokens.CacheReadTokens += msg.Usage.CacheReadInputTokens
		p.cumulativeTokens.CacheWriteTokens += msg.Usage.CacheCreationInputTokens

		events = append(events, agent.ParsedEvent{
			Type:             agent.EventTokenUpdate,
			SessionID:        entry.SessionID,
			Timestamp:        entry.Timestamp,
			Model:            p.lastModel,
			Tokens:           p.cumulativeTokens,
			EstimatedCostUSD: EstimateCost(p.lastModel, p.cumulativeTokens),
		})
	}

	// Extract tool use events from content blocks.
	for _, block := range msg.Content.Blocks {
		if block.Type == "tool_use" && block.Name != "" {
			events = append(events, agent.ParsedEvent{
				Type:      agent.EventToolUse,
				SessionID: entry.SessionID,
				Timestamp: entry.Timestamp,
				Model:     p.lastModel,
				ToolName:  block.Name,
				ToolInput: truncateToolInput(block.Input),
			})
		}
	}

	// Emit turn complete when the model stops.
	if msg.StopReason == "end_turn" {
		events = append(events, agent.ParsedEvent{
			Type:      agent.EventTurnComplete,
			SessionID: entry.SessionID,
			Timestamp: entry.Timestamp,
			Model:     p.lastModel,
		})
	}

	return events
}

// parseUser handles user-type JSONL entries. Produces UserPrompt events
// for direct user messages (not tool results).
func (p *Parser) parseUser(entry SessionEntry) []agent.ParsedEvent {
	if entry.Message == nil {
		return nil
	}

	// Tool results (content is an array with tool_result blocks) are not user prompts.
	if len(entry.Message.Content.Blocks) > 0 {
		return nil
	}

	// Plain text user messages.
	promptText := entry.Message.Content.Text
	if promptText == "" {
		return nil
	}

	return []agent.ParsedEvent{{
		Type:      agent.EventUserPrompt,
		SessionID: entry.SessionID,
		Timestamp: entry.Timestamp,
		Prompt:    truncateString(promptText, 500),
		GitBranch: entry.GitBranch,
	}}
}

// parseSystem handles system-type JSONL entries. Produces TurnDuration events
// for "turn_duration" subtypes.
func (p *Parser) parseSystem(entry SessionEntry) []agent.ParsedEvent {
	switch entry.Subtype {
	case "turn_duration":
		return []agent.ParsedEvent{{
			Type:       agent.EventTurnDuration,
			SessionID:  entry.SessionID,
			Timestamp:  entry.Timestamp,
			DurationMs: entry.DurationMs,
		}}
	default:
		return nil
	}
}

// truncateToolInput serializes tool input to a summary string, truncated
// to maxToolInputLength characters.
func truncateToolInput(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return truncateString(string(data), maxToolInputLength)
}

// truncateString caps a string at maxLen characters, appending "…" if truncated.
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "…"
}

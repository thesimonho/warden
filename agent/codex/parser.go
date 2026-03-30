package codex

import (
	"encoding/json"
	"fmt"

	"github.com/thesimonho/warden/agent"
)

// maxToolInputLength is the maximum length of tool input included in events.
const maxToolInputLength = 1000

// maxPromptLength is the maximum length of user prompt text included in events.
const maxPromptLength = 500

// Parser implements agent.SessionParser for Codex CLI session JSONL files.
// Token counts in Codex are cumulative (total_token_usage), so the parser
// forwards them directly without accumulating.
type Parser struct {
	// lastModel tracks the most recently seen model from turn_context entries.
	lastModel string
	// sessionID is the session identifier from session_meta.
	sessionID string
}

// NewParser creates a new Codex JSONL parser.
func NewParser() *Parser {
	return &Parser{}
}

// ParseLine parses a single JSONL line into zero or more ParsedEvents.
func (p *Parser) ParseLine(line []byte) []agent.ParsedEvent {
	var item RolloutItem
	if err := json.Unmarshal(line, &item); err != nil {
		return nil
	}

	switch item.Type {
	case "session_meta":
		return p.parseSessionMeta(item)
	case "turn_context":
		return p.parseTurnContext(item)
	case "response_item":
		return p.parseResponseItem(item)
	case "event_msg":
		return p.parseEventMsg(item)
	default:
		return nil
	}
}

// SessionDir returns the host-side directory containing Codex session JSONL files.
// Codex stores all sessions under ~/.codex/sessions/ in date-based subdirectories.
// Unlike Claude (one directory per project), Codex uses a flat date hierarchy.
func (p *Parser) SessionDir(homeDir string, _ agent.ProjectInfo) string {
	return fmt.Sprintf("%s/.codex/sessions", homeDir)
}

// parseSessionMeta extracts session identity and git info.
func (p *Parser) parseSessionMeta(item RolloutItem) []agent.ParsedEvent {
	var meta SessionMeta
	if err := json.Unmarshal(item.Payload, &meta); err != nil {
		return nil
	}

	p.sessionID = meta.ID

	event := agent.ParsedEvent{
		Type:      agent.EventSessionStart,
		SessionID: meta.ID,
		Timestamp: item.Timestamp,
	}
	if meta.Git != nil {
		event.GitBranch = meta.Git.Branch
	}

	return []agent.ParsedEvent{event}
}

// parseTurnContext extracts model and approval policy info.
func (p *Parser) parseTurnContext(item RolloutItem) []agent.ParsedEvent {
	var ctx TurnContext
	if err := json.Unmarshal(item.Payload, &ctx); err != nil {
		return nil
	}

	if ctx.Model != "" {
		p.lastModel = ctx.Model
	}

	// Turn context is informational — no event emitted.
	return nil
}

// parseResponseItem handles function calls (tool use) and messages.
func (p *Parser) parseResponseItem(item RolloutItem) []agent.ParsedEvent {
	var resp ResponseItem
	if err := json.Unmarshal(item.Payload, &resp); err != nil {
		return nil
	}

	switch resp.Type {
	case "function_call":
		return []agent.ParsedEvent{{
			Type:      agent.EventToolUse,
			SessionID: p.sessionID,
			Timestamp: item.Timestamp,
			Model:     p.lastModel,
			ToolName:  resp.Name,
			ToolInput: truncateString(resp.Arguments, maxToolInputLength),
		}}
	default:
		// message, reasoning, function_call_output — no events.
		return nil
	}
}

// parseEventMsg handles token counts, user messages, and task lifecycle.
func (p *Parser) parseEventMsg(item RolloutItem) []agent.ParsedEvent {
	var msg EventMsg
	if err := json.Unmarshal(item.Payload, &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "user_message":
		if msg.Message == "" {
			return nil
		}
		return []agent.ParsedEvent{{
			Type:      agent.EventUserPrompt,
			SessionID: p.sessionID,
			Timestamp: item.Timestamp,
			Prompt:    truncateString(msg.Message, maxPromptLength),
		}}

	case "token_count":
		if msg.Info == nil {
			return nil
		}
		total := msg.Info.TotalTokenUsage
		tokens := agent.TokenUsage{
			InputTokens:      total.InputTokens,
			OutputTokens:      total.OutputTokens + total.ReasoningOutputTokens,
			CacheReadTokens:  total.CachedInputTokens,
			CacheWriteTokens: 0, // Codex doesn't report cache writes separately.
		}
		return []agent.ParsedEvent{{
			Type:             agent.EventTokenUpdate,
			SessionID:        p.sessionID,
			Timestamp:        item.Timestamp,
			Model:            p.lastModel,
			Tokens:           tokens,
			EstimatedCostUSD: EstimateCost(p.lastModel, tokens),
		}}

	case "task_complete":
		return []agent.ParsedEvent{{
			Type:      agent.EventTurnComplete,
			SessionID: p.sessionID,
			Timestamp: item.Timestamp,
			Model:     p.lastModel,
		}}

	case "task_started":
		// Informational — no event.
		return nil

	default:
		return nil
	}
}

// truncateString caps a string at maxLen runes, appending "…" if truncated.
func truncateString(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

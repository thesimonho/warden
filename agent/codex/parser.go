package codex

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thesimonho/warden/agent"
)

// Parser implements agent.SessionParser for Codex CLI session JSONL files.
// Token counts in Codex are cumulative (total_token_usage), so the parser
// forwards them directly without accumulating.
type Parser struct {
	// lastModel tracks the most recently seen model from turn_context entries.
	lastModel string
	// sessionID is the session identifier from session_meta.
	sessionID string
	// worktreeID is derived from the session_meta CWD.
	worktreeID string
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
	return filepath.Join(homeDir, ".codex", "sessions")
}

// FindSessionFiles discovers active Codex session files for a project by
// reading shell_snapshots. Codex creates a shell snapshot file per active
// session at ~/.codex/shell_snapshots/<session_id>.<timestamp>.sh. Each
// snapshot contains exported env vars including WARDEN_PROJECT_ID, which
// we use to filter to the correct project. The session ID from the filename
// is then used to glob for the matching JSONL file across date directories.
func (p *Parser) FindSessionFiles(homeDir string, project agent.ProjectInfo) []string {
	snapshotDir := filepath.Join(homeDir, ".codex", "shell_snapshots")
	entries, err := os.ReadDir(snapshotDir)
	if err != nil {
		return nil
	}

	// Codex shell snapshots use `declare -x KEY="value"` format.
	projectMarker := []byte(fmt.Sprintf("WARDEN_PROJECT_ID=%q", project.ProjectID))
	sessionsDir := filepath.Join(homeDir, ".codex", "sessions")
	var files []string

	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sh") {
			continue
		}

		// Extract session ID from filename: <session_id>.<timestamp>.sh
		sessionID, _, found := strings.Cut(e.Name(), ".")
		if !found || sessionID == "" {
			continue
		}

		// Check if this snapshot belongs to our project.
		data, err := os.ReadFile(filepath.Join(snapshotDir, e.Name()))
		if err != nil {
			continue
		}
		if !bytes.Contains(data, projectMarker) {
			continue
		}

		// Glob for the JSONL file by session ID across all date directories.
		pattern := filepath.Join(sessionsDir, "*", "*", "*", "rollout-*-"+sessionID+".jsonl")
		matches, err := filepath.Glob(pattern)
		if err != nil || len(matches) == 0 {
			continue
		}
		files = append(files, matches[0])
	}

	return files
}

// parseSessionMeta extracts session identity and git info.
func (p *Parser) parseSessionMeta(item RolloutItem) []agent.ParsedEvent {
	var meta SessionMeta
	if err := json.Unmarshal(item.Payload, &meta); err != nil {
		return nil
	}

	p.sessionID = meta.ID
	p.worktreeID = agent.WorktreeIDFromCWD(meta.CWD, "")

	event := agent.ParsedEvent{
		Type:       agent.EventSessionStart,
		SessionID:  meta.ID,
		WorktreeID: p.worktreeID,
		Timestamp:  item.Timestamp,
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
			Type:       agent.EventToolUse,
			SessionID:  p.sessionID,
			WorktreeID: p.worktreeID,
			Timestamp:  item.Timestamp,
			Model:      p.lastModel,
			ToolName:   resp.Name,
			ToolInput:  agent.TruncateString(resp.Arguments, agent.MaxToolInputLength),
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
			Type:       agent.EventUserPrompt,
			SessionID:  p.sessionID,
			WorktreeID: p.worktreeID,
			Timestamp:  item.Timestamp,
			Prompt:     agent.TruncateString(msg.Message, agent.MaxPromptLength),
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
			WorktreeID:       p.worktreeID,
			Timestamp:        item.Timestamp,
			Model:            p.lastModel,
			Tokens:           tokens,
			EstimatedCostUSD: EstimateCost(p.lastModel, tokens),
		}}

	case "task_complete":
		return []agent.ParsedEvent{{
			Type:       agent.EventTurnComplete,
			SessionID:  p.sessionID,
			WorktreeID: p.worktreeID,
			Timestamp:  item.Timestamp,
			Model:      p.lastModel,
		}}

	case "task_started":
		// Informational — no event.
		return nil

	default:
		return nil
	}
}


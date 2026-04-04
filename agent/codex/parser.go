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

// likelyErrorPrefixes are lowercase prefixes that indicate a function_call_output
// is an error. Codex doesn't have a dedicated is_error field on outputs.
// False positives (e.g. "not found any issues") produce spurious ToolUseFailure
// audit events — acceptable since the audit log is informational, not authoritative.
var likelyErrorPrefixes = []string{"error:", "exit code ", "command failed", "permission denied", "not found"}

// isLikelyError checks if a function_call_output looks like an error
// by matching the start of the output against common error prefixes.
func isLikelyError(output string) bool {
	if len(output) == 0 {
		return false
	}
	// Only lowercase the prefix window to avoid allocating for the full output.
	const maxPrefixLen = 20
	end := min(len(output), maxPrefixLen)
	lower := strings.ToLower(output[:end])
	for _, p := range likelyErrorPrefixes {
		if strings.HasPrefix(lower, p) {
			return true
		}
	}
	return false
}

// Parser implements agent.SessionParser for Codex CLI session JSONL files.
// Token counts in Codex are cumulative (total_token_usage), so the parser
// forwards them directly without accumulating.
//
// Codex's rollout persistence policy filters which events land in JSONL.
// In limited mode (CLI default), begin events (exec_command_begin,
// mcp_tool_call_begin, patch_apply_begin), approval/permission requests,
// elicitation requests, and stream errors are never persisted. Tool use is
// captured via response_item entries (always persisted). Error details from
// end events (exec_command_end, etc.) require extended persistence mode,
// only available via `codex app-server` (ThreadStartParams.persist_extended_history).
// See docs/events_codex.md for the full persistence policy.
type Parser struct {
	// lastModel tracks the most recently seen model from turn_context entries.
	lastModel string
	// sessionID is the session identifier from session_meta.
	sessionID string
	// worktreeID is derived from the session_meta CWD.
	worktreeID string
	// toolNames maps call IDs to tool names for correlating errors.
	toolNames map[string]string
}

// NewParser creates a new Codex JSONL parser.
func NewParser() *Parser {
	return &Parser{
		toolNames: make(map[string]string),
	}
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
	case "compacted":
		return p.parseCompacted(item)
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
	p.worktreeID = agent.WorktreeIDFromCWD(meta.CWD)

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

// toolUseEvent builds a tool_use ParsedEvent with common fields pre-filled.
func (p *Parser) toolUseEvent(ts, toolName, toolInput string) agent.ParsedEvent {
	return agent.ParsedEvent{
		Type:       agent.EventToolUse,
		SessionID:  p.sessionID,
		WorktreeID: p.worktreeID,
		Timestamp:  ts,
		Model:      p.lastModel,
		ToolName:   toolName,
		ToolInput:  toolInput,
	}
}

// toolFailureEvent builds a tool_use_failure ParsedEvent with common fields pre-filled.
func (p *Parser) toolFailureEvent(ts, toolName, errorContent string) agent.ParsedEvent {
	return agent.ParsedEvent{
		Type:         agent.EventToolUseFailure,
		SessionID:    p.sessionID,
		WorktreeID:   p.worktreeID,
		Timestamp:    ts,
		ToolName:     toolName,
		ErrorContent: errorContent,
	}
}

// parseUserShellCommand produces user_prompt events from a user-initiated
// shell command (exec_command_end with source == "user_shell"). Emits a bash
// command event and, if output is non-empty, a bash_output event — matching
// Claude Code's audit format for ! bash mode.
func (p *Parser) parseUserShellCommand(item RolloutItem, msg EventMsg) []agent.ParsedEvent {
	cmdStr := extractUserCommand(msg.Command)
	if cmdStr == "" {
		return nil
	}

	events := []agent.ParsedEvent{{
		Type:         agent.EventUserPrompt,
		SessionID:    p.sessionID,
		WorktreeID:   p.worktreeID,
		Timestamp:    item.Timestamp,
		Prompt:       agent.TruncateString("$ "+cmdStr, agent.MaxPromptLength),
		PromptSource: agent.PromptSourceBash,
	}}

	// Prefer aggregated_output (always populated) over separate stdout/stderr
	// (often empty when Codex captures combined output).
	output := strings.TrimSpace(msg.AggregatedOutput)
	if output == "" {
		output = strings.TrimSpace(msg.Stdout + msg.Stderr)
	}
	if output != "" {
		events = append(events, agent.ParsedEvent{
			Type:         agent.EventUserPrompt,
			SessionID:    p.sessionID,
			WorktreeID:   p.worktreeID,
			Timestamp:    item.Timestamp,
			Prompt:       agent.TruncateString(output, agent.MaxPromptLength),
			PromptSource: agent.PromptSourceBashOutput,
		})
	}

	return events
}

// extractUserCommand returns the meaningful command string from a Codex
// user shell command array. Codex wraps commands as ["/bin/bash", "-lc", "<cmd>"],
// so we extract the last argument when this pattern is detected.
func extractUserCommand(command []string) string {
	if len(command) >= 3 && strings.HasSuffix(command[0], "bash") && command[1] == "-lc" {
		return strings.Join(command[2:], " ")
	}
	return strings.Join(command, " ")
}

// parseResponseItem handles function calls (tool use) and messages.
// All response_item types are always persisted to JSONL (except "other").
func (p *Parser) parseResponseItem(item RolloutItem) []agent.ParsedEvent {
	var resp ResponseItem
	if err := json.Unmarshal(item.Payload, &resp); err != nil {
		return nil
	}

	switch resp.Type {
	case "function_call":
		if resp.CallID != "" && resp.Name != "" {
			p.toolNames[resp.CallID] = resp.Name
		}
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, resp.Name, agent.TruncateString(resp.Arguments, agent.MaxToolInputLength))}
	case "function_call_output":
		if resp.Status == "incomplete" || resp.Output == "" {
			return nil
		}
		// Heuristic: outputs starting with error indicators are failures.
		// Codex doesn't have is_error — check for common error patterns.
		if !isLikelyError(resp.Output) {
			return nil
		}
		toolName := p.toolNames[resp.CallID]
		delete(p.toolNames, resp.CallID)
		return []agent.ParsedEvent{p.toolFailureEvent(item.Timestamp, toolName, agent.TruncateString(resp.Output, agent.MaxToolInputLength))}
	case "local_shell_call":
		toolInput := ""
		if resp.Action != nil && len(resp.Action.Command) > 0 {
			toolInput = strings.Join(resp.Action.Command, " ")
		}
		if resp.CallID != "" {
			p.toolNames[resp.CallID] = "shell"
		}
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, "shell", agent.TruncateString(toolInput, agent.MaxToolInputLength))}
	case "web_search_call":
		query := ""
		if resp.Action != nil {
			query = resp.Action.Query
		}
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, "web_search", agent.TruncateString(query, agent.MaxToolInputLength))}
	case "custom_tool_call":
		if resp.CallID != "" && resp.Name != "" {
			p.toolNames[resp.CallID] = resp.Name
		}
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, resp.Name, agent.TruncateString(resp.Input, agent.MaxToolInputLength))}
	case "image_generation_call":
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, "image_generation", "")}
	case "tool_search_call":
		return []agent.ParsedEvent{p.toolUseEvent(item.Timestamp, "tool_search", "")}
	default:
		// message, reasoning, compaction, etc. — no events.
		return nil
	}
}

// parseEventMsg handles token counts, user messages, and task lifecycle.
//
// Persistence note: many event_msg types are never written to JSONL in limited
// mode (CLI default). The cases below only handle events that are persisted in
// limited or extended mode. Events only available via codex app-server (never
// persisted): exec_approval_request, request_permissions, elicitation_request,
// exec_command_begin, mcp_tool_call_begin, patch_apply_begin, stream_error.
func (p *Parser) parseEventMsg(item RolloutItem) []agent.ParsedEvent {
	var msg EventMsg
	if err := json.Unmarshal(item.Payload, &msg); err != nil {
		return nil
	}

	switch msg.Type {
	case "user_message":
		formatted := agent.FormatPromptText(msg.Message)
		if formatted.Text == "" {
			return nil
		}
		return []agent.ParsedEvent{{
			Type:         agent.EventUserPrompt,
			SessionID:    p.sessionID,
			WorktreeID:   p.worktreeID,
			Timestamp:    item.Timestamp,
			Prompt:       agent.TruncateString(formatted.Text, agent.MaxPromptLength),
			PromptSource: formatted.Source,
		}}

	case "token_count":
		if msg.Info == nil {
			return nil
		}
		total := msg.Info.TotalTokenUsage
		tokens := agent.TokenUsage{
			InputTokens:      total.InputTokens,
			OutputTokens:     total.OutputTokens + total.ReasoningOutputTokens,
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

	case "task_complete", "turn_complete":
		// Clear stale call ID → tool name mappings from the completed turn.
		clear(p.toolNames)
		return []agent.ParsedEvent{{
			Type:       agent.EventTurnComplete,
			SessionID:  p.sessionID,
			WorktreeID: p.worktreeID,
			Timestamp:  item.Timestamp,
			Model:      p.lastModel,
		}}

	case "task_started", "turn_started":
		// Informational — no event emitted.
		return nil

	// Extended mode only — requires persist_extended_history (app-server).
	case "error":
		return []agent.ParsedEvent{{
			Type:         agent.EventStopFailure,
			SessionID:    p.sessionID,
			WorktreeID:   p.worktreeID,
			Timestamp:    item.Timestamp,
			ErrorContent: agent.TruncateString(msg.Message, agent.MaxToolInputLength),
		}}

	// Limited mode — always persisted.
	case "turn_aborted":
		return []agent.ParsedEvent{{
			Type:         agent.EventStopFailure,
			SessionID:    p.sessionID,
			WorktreeID:   p.worktreeID,
			Timestamp:    item.Timestamp,
			ErrorContent: fmt.Sprintf("turn aborted: %s", msg.Reason),
		}}

	case "context_compacted":
		return []agent.ParsedEvent{{
			Type:           agent.EventContextCompact,
			SessionID:      p.sessionID,
			WorktreeID:     p.worktreeID,
			Timestamp:      item.Timestamp,
			CompactTrigger: "context_compacted",
		}}

	case "thread_rolled_back":
		return []agent.ParsedEvent{{
			Type:           agent.EventContextCompact,
			SessionID:      p.sessionID,
			WorktreeID:     p.worktreeID,
			Timestamp:      item.Timestamp,
			CompactTrigger: "rollback",
			Content:        fmt.Sprintf("rolled back %d turns", msg.NumTurns),
		}}

	// User shell commands (source == "user_shell") are persisted in limited mode.
	// Agent-initiated commands are extended mode only — exit code errors.
	case "exec_command_end":
		if msg.Source == "user_shell" {
			return p.parseUserShellCommand(item, msg)
		}
		if msg.ExitCode != nil && *msg.ExitCode != 0 {
			return []agent.ParsedEvent{p.toolFailureEvent(item.Timestamp, "exec_command", fmt.Sprintf("exit code %d", *msg.ExitCode))}
		}
		return nil

	case "mcp_tool_call_end":
		if msg.Status == "error" {
			return []agent.ParsedEvent{p.toolFailureEvent(item.Timestamp, msg.ToolName, msg.Message)}
		}
		return nil

	case "patch_apply_end":
		if msg.Status == "error" {
			return []agent.ParsedEvent{p.toolFailureEvent(item.Timestamp, "patch_apply", msg.Message)}
		}
		return nil

	default:
		return nil
	}
}

// parseCompacted handles top-level "compacted" entries (conversation compaction markers).
func (p *Parser) parseCompacted(item RolloutItem) []agent.ParsedEvent {
	var compacted CompactedItem
	if err := json.Unmarshal(item.Payload, &compacted); err != nil {
		return nil
	}
	return []agent.ParsedEvent{{
		Type:           agent.EventContextCompact,
		SessionID:      p.sessionID,
		WorktreeID:     p.worktreeID,
		Timestamp:      item.Timestamp,
		CompactTrigger: "compacted",
		Content:        compacted.Message,
	}}
}

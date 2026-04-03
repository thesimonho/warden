package claudecode

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/thesimonho/warden/agent"
)

// Parser implements agent.SessionParser for Claude Code session JSONL files.
// It is stateful — token counts accumulate across lines within a session.
// Create a new Parser for each session file.
type Parser struct {
	// cumulativeTokens tracks running totals across the session.
	cumulativeTokens agent.TokenUsage
	// lastModel tracks the most recently seen model name.
	lastModel string
	// toolNames maps tool_use IDs to tool names for correlating tool_result errors.
	toolNames map[string]string
}

// NewParser creates a new Claude Code JSONL parser.
func NewParser() *Parser {
	return &Parser{
		toolNames: make(map[string]string),
	}
}

// ParseLine parses a single JSONL line into zero or more ParsedEvents.
// Returns nil for entry types that don't map to Warden events.
func (p *Parser) ParseLine(line []byte) []agent.ParsedEvent {
	var entry SessionEntry
	if err := json.Unmarshal(line, &entry); err != nil {
		return nil
	}

	var events []agent.ParsedEvent
	switch entry.Type {
	case "assistant":
		events = p.parseAssistant(entry)
	case "user":
		events = p.parseUser(entry)
	case "system":
		events = p.parseSystem(entry)
	case "queue-operation":
		events = parseQueueOperation(entry)
	default:
		// file-history-snapshot, last-prompt — no events.
		return nil
	}

	// Inject worktree ID from the entry's CWD. Each JSONL entry carries
	// the container-side working directory, which includes the worktree
	// path for non-main worktrees.
	if entry.CWD != "" {
		worktreeID := agent.WorktreeIDFromCWD(entry.CWD)
		for i := range events {
			if events[i].WorktreeID == "" {
				events[i].WorktreeID = worktreeID
			}
		}
	}

	return events
}

// SessionDir returns the host-side directory containing Claude Code session
// JSONL files for a project. Claude encodes the container-side workspace
// path by replacing "/" with "-" to form the directory name.
func (p *Parser) SessionDir(homeDir string, project agent.ProjectInfo) string {
	encoded := encodeWorkspacePath(project.WorkspaceDir)
	return filepath.Join(homeDir, ".claude", "projects", encoded)
}

// FindSessionFiles scans the per-project session directory and any
// worktree session directories for .jsonl files. Claude Code creates
// separate session directories for worktrees at paths like:
//
//	~/.claude/projects/-home-warden-myproject--claude-worktrees-branchname/
//
// These are siblings of the project directory and share the same prefix.
func (p *Parser) FindSessionFiles(homeDir string, project agent.ProjectInfo) []string {
	projectDir := p.SessionDir(homeDir, project)
	var files []string

	// Scan the main project session directory.
	files = append(files, scanJSONLFiles(projectDir)...)

	// Scan worktree session directories (siblings with matching prefix).
	worktreePattern := projectDir + "--claude-worktrees-*"
	worktreeDirs, err := filepath.Glob(worktreePattern)
	if err != nil {
		slog.Warn("failed to glob worktree session dirs", "pattern", worktreePattern, "err", err)
	}
	for _, wtDir := range worktreeDirs {
		files = append(files, scanJSONLFiles(wtDir)...)
	}

	return files
}

// scanJSONLFiles returns absolute paths of .jsonl files in a directory.
func scanJSONLFiles(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".jsonl") {
			files = append(files, filepath.Join(dir, e.Name()))
		}
	}
	return files
}

// encodeWorkspacePath converts a container workspace path to Claude's
// directory encoding: replace "/" with "-".
// Example: "/home/warden/my-project" → "-home-warden-my-project"
func encodeWorkspacePath(workspaceDir string) string {
	return strings.ReplaceAll(workspaceDir, "/", "-")
}

// parseQueueOperation handles queue-operation JSONL entries. Enqueued prompts
// are user messages submitted while Claude is still working — they should be
// logged as user_prompt events just like regular user messages.
func parseQueueOperation(entry SessionEntry) []agent.ParsedEvent {
	if entry.Operation != "enqueue" || entry.Content == "" {
		return nil
	}
	return []agent.ParsedEvent{{
		Type:      agent.EventUserPrompt,
		SessionID: entry.SessionID,
		Timestamp: entry.Timestamp,
		Prompt:    agent.TruncateString(entry.Content, agent.MaxPromptLength),
	}}
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

	// Extract tool use events from content blocks and track IDs for error correlation.
	for _, block := range msg.Content.Blocks {
		if block.Type == "tool_use" && block.Name != "" {
			if block.ID != "" {
				p.toolNames[block.ID] = block.Name
			}
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
// for direct user messages, and ToolUseFailure events for error tool results.
func (p *Parser) parseUser(entry SessionEntry) []agent.ParsedEvent {
	if entry.Message == nil {
		return nil
	}

	// Content with blocks: check for tool_result errors.
	if len(entry.Message.Content.Blocks) > 0 {
		return p.parseToolResults(entry)
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
		Prompt:    agent.TruncateString(promptText, agent.MaxPromptLength),
		GitBranch: entry.GitBranch,
	}}
}

// parseToolResults extracts ToolUseFailure events from tool_result blocks
// that have is_error set. The tool name is resolved from the preceding
// tool_use block via the tool_use_id → tool name mapping.
func (p *Parser) parseToolResults(entry SessionEntry) []agent.ParsedEvent {
	var events []agent.ParsedEvent
	for _, block := range entry.Message.Content.Blocks {
		if block.Type != "tool_result" || !block.IsError {
			continue
		}
		toolName := p.toolNames[block.ToolUseID]
		delete(p.toolNames, block.ToolUseID)
		events = append(events, agent.ParsedEvent{
			Type:         agent.EventToolUseFailure,
			SessionID:    entry.SessionID,
			Timestamp:    entry.Timestamp,
			ToolName:     toolName,
			ErrorContent: agent.TruncateString(block.ErrorContent(), agent.MaxToolInputLength),
		})
	}
	return events
}

// parseSystem handles system-type JSONL entries. Each subtype maps to a
// specific ParsedEventType — see docs/events_claude.md for the full catalog.
func (p *Parser) parseSystem(entry SessionEntry) []agent.ParsedEvent {
	switch entry.Subtype {
	case "turn_duration":
		return []agent.ParsedEvent{{
			Type:       agent.EventTurnDuration,
			SessionID:  entry.SessionID,
			Timestamp:  entry.Timestamp,
			DurationMs: entry.DurationMs,
		}}
	case "api_error":
		return []agent.ParsedEvent{{
			Type:         agent.EventStopFailure,
			SessionID:    entry.SessionID,
			Timestamp:    entry.Timestamp,
			ErrorContent: agent.TruncateString(entry.Content, agent.MaxToolInputLength),
		}}
	case "agents_killed":
		return []agent.ParsedEvent{{
			Type:      agent.EventSubagentStop,
			SessionID: entry.SessionID,
			Timestamp: entry.Timestamp,
			Content:   entry.Content,
		}}
	case "api_metrics":
		return []agent.ParsedEvent{{
			Type:               agent.EventApiMetrics,
			SessionID:          entry.SessionID,
			Timestamp:          entry.Timestamp,
			TTFTMs:             entry.TTFTMs,
			OutputTokensPerSec: entry.OutputTokensPerSec,
		}}
	case "permission_retry":
		return []agent.ParsedEvent{{
			Type:      agent.EventPermissionGrant,
			SessionID: entry.SessionID,
			Timestamp: entry.Timestamp,
			Commands:  entry.Commands,
			Content:   entry.Content,
		}}
	case "compact_boundary", "microcompact_boundary":
		event := agent.ParsedEvent{
			Type:      agent.EventContextCompact,
			SessionID: entry.SessionID,
			Timestamp: entry.Timestamp,
			Content:   entry.Content,
		}
		if entry.CompactMetadata != nil {
			event.CompactTrigger = entry.CompactMetadata.Trigger
			event.PreCompactTokens = entry.CompactMetadata.PreTokens
		}
		return []agent.ParsedEvent{event}
	case "stop_hook_summary", "away_summary", "memory_saved",
		"bridge_status", "local_command", "informational",
		"scheduled_task_fire":
		return []agent.ParsedEvent{{
			Type:      agent.EventSystemInfo,
			SessionID: entry.SessionID,
			Timestamp: entry.Timestamp,
			Subtype:   entry.Subtype,
			Content:   entry.Content,
		}}
	default:
		return nil
	}
}

// truncateToolInput serializes tool input to a summary string, truncated
// to agent.MaxToolInputLength characters.
func truncateToolInput(input map[string]any) string {
	if len(input) == 0 {
		return ""
	}
	data, err := json.Marshal(input)
	if err != nil {
		return ""
	}
	return agent.TruncateString(string(data), agent.MaxToolInputLength)
}

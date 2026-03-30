package agent

// SessionParser parses agent-specific JSONL session files into
// agent-agnostic ParsedEvents. Each agent implementation (Claude Code,
// Codex) provides its own parser that knows the JSONL schema.
//
// Parsers are stateful — they accumulate token counts across lines
// within a session. Create a new parser per session file.
type SessionParser interface {
	// ParseLine parses a single JSONL line into zero or more Warden events.
	// Returns nil for lines that don't produce events (e.g. file-history-snapshot).
	ParseLine(line []byte) []ParsedEvent

	// SessionDir returns the host-side directory to watch for session files.
	// The directory path is constructed from the host home dir and project metadata.
	SessionDir(homeDir string, project ProjectInfo) string
}

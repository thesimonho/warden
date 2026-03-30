package agent

// StatusProvider extracts agent status data from a config file that the agent
// writes inside the container. Each agent implementation knows where its data
// lives and how to parse it.
//
// The Docker layer reads the file via exec and passes raw bytes here.
// The provider returns status data keyed by working directory path, which the
// caller uses to match against dashboard sessions.
type StatusProvider interface {
	// Name returns a human-readable agent identifier (e.g. "claude-code").
	Name() string

	// ProcessName returns the CLI binary name used for pgrep process detection
	// (e.g. "claude", "codex"). This is the executable name, not the agent type.
	ProcessName() string

	// ConfigFilePath returns the absolute path to the agent's config file
	// inside the container (e.g. "/home/dev/.claude.json").
	ConfigFilePath() string

	// ExtractStatus parses the config file contents and returns status data
	// keyed by the working directory path that the agent was running in.
	//
	// For agents that use worktrees, the key is the worktree path
	// (e.g. "/project/.claude/worktrees/abc-123"). For non-worktree sessions,
	// the key is the project root (e.g. "/project").
	//
	// Returns nil for keys where no status data is available.
	// Returns an empty map if the config data is empty or unparseable.
	ExtractStatus(configData []byte) map[string]*Status

	// NewSessionParser creates a new stateful parser for JSONL session files.
	// Each parser instance accumulates state (e.g. token counts) across lines,
	// so a new parser should be created per session file. Returns nil if the
	// provider does not support JSONL parsing.
	NewSessionParser() SessionParser
}

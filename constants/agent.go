package constants

// AgentType identifies an AI coding agent supported by Warden.
// Used as the database column value and WARDEN_AGENT_TYPE env var.
type AgentType string

const (
	// AgentClaudeCode identifies Anthropic's Claude Code CLI.
	AgentClaudeCode AgentType = "claude-code"
	// AgentCodex identifies OpenAI's Codex CLI.
	AgentCodex AgentType = "codex"

	// DefaultAgentType is the agent type used when none is specified.
	DefaultAgentType = AgentClaudeCode
)

// Valid reports whether the agent type is a known supported type.
func (t AgentType) Valid() bool {
	return t == AgentClaudeCode || t == AgentCodex
}

// String returns the string representation of the agent type.
func (t AgentType) String() string {
	return string(t)
}

// AllAgentTypes lists all supported agent type identifiers in display order.
var AllAgentTypes = []AgentType{AgentClaudeCode, AgentCodex}

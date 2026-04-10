package agent

// Pinned CLI versions for container installation.
//
// These are the single source of truth — the container startup script
// installs the exact version specified here. CI bumps these constants
// after parser compatibility tests pass for the new version.
//
// The JSONL parser is tightly coupled to CLI output format, so pinning
// prevents breakage from unvalidated upstream changes.
import "github.com/thesimonho/warden/constants"

const (
	// ClaudeCodeVersion is the pinned Claude Code CLI version.
	// Query latest: curl -sfL "https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases/latest"
	ClaudeCodeVersion = "2.1.98"

	// CodexVersion is the pinned OpenAI Codex CLI version.
	// Query latest: npm view @openai/codex version
	CodexVersion = "0.118.0"
)

// VersionForType returns the pinned CLI version for the given agent type.
func VersionForType(agentType constants.AgentType) string {
	switch agentType {
	case ClaudeCode:
		return ClaudeCodeVersion
	case Codex:
		return CodexVersion
	default:
		return ""
	}
}

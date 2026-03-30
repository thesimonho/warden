// Package codex implements the agent.StatusProvider and agent.SessionParser
// interfaces for OpenAI Codex CLI. It parses session JSONL files written
// by Codex at ~/.codex/sessions/YYYY/MM/DD/rollout-*.jsonl.
//
// Unlike Claude Code, Codex has no separate config file with cost data.
// All cost information comes from token-based estimation via the JSONL parser.
package codex

import "github.com/thesimonho/warden/agent"

// Provider implements agent.StatusProvider for Codex CLI.
// Codex has no config file equivalent to .claude.json, so ExtractStatus
// returns an empty map. Cost comes from JSONL parsing via EstimateCost.
type Provider struct{}

// NewProvider creates a new Codex status provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return "codex"
}

// ProcessName returns the CLI binary name for pgrep detection.
func (p *Provider) ProcessName() string {
	return "codex"
}

// ConfigFilePath returns an empty path — Codex has no global config file
// equivalent to Claude's .claude.json.
func (p *Provider) ConfigFilePath() string {
	return ""
}

// ExtractStatus returns an empty map — Codex doesn't write per-project
// status to a config file. Cost data comes from JSONL token events.
func (p *Provider) ExtractStatus(_ []byte) map[string]*agent.Status {
	return map[string]*agent.Status{}
}

// NewSessionParser creates a stateful JSONL parser for Codex sessions.
func (p *Provider) NewSessionParser() agent.SessionParser {
	return NewParser()
}

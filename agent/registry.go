package agent

import (
	"sync"

	"github.com/thesimonho/warden/constants"
)

// Type aliases for convenience — re-export from constants so existing
// callers (agent.ClaudeCode, agent.DefaultType, etc.) still work.
const (
	ClaudeCode  = constants.AgentClaudeCode
	Codex       = constants.AgentCodex
	DefaultType = constants.DefaultAgentType
)

// AllTypes lists all supported agent type identifiers in display order.
var AllTypes = constants.AllAgentTypes

// DisplayLabels maps agent type identifiers to human-readable labels.
var DisplayLabels = map[constants.AgentType]string{
	ClaudeCode: "Claude Code",
	Codex:      "OpenAI Codex",
}

// ShortLabel returns a compact display label for the given agent type,
// falling back to the type string itself when no mapping exists.
func ShortLabel(agentType constants.AgentType) string {
	switch agentType {
	case ClaudeCode:
		return "claude"
	case Codex:
		return "codex"
	default:
		return string(agentType)
	}
}

// Registry holds StatusProvider instances keyed by agent type.
// It allows the engine to resolve the correct provider for a container
// based on its WARDEN_AGENT_TYPE env var.
type Registry struct {
	mu        sync.RWMutex
	providers map[constants.AgentType]StatusProvider
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[constants.AgentType]StatusProvider),
	}
}

// Register adds a StatusProvider for the given agent type.
func (r *Registry) Register(agentType constants.AgentType, provider StatusProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[agentType] = provider
}

// Get returns the StatusProvider for the given agent type.
// Returns nil and false if no provider is registered for that type.
func (r *Registry) Get(agentType constants.AgentType) (StatusProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[agentType]
	return p, ok
}

// Default returns the StatusProvider for the default agent type (claude-code).
// Returns nil if the default provider is not registered.
func (r *Registry) Default() StatusProvider {
	p, _ := r.Get(DefaultType)
	return p
}

// Resolve returns the StatusProvider for the given agent type, falling
// back to the default provider if the type is empty or unregistered.
func (r *Registry) Resolve(agentType constants.AgentType) StatusProvider {
	if agentType == "" {
		return r.Default()
	}
	if p, ok := r.Get(agentType); ok {
		return p
	}
	return r.Default()
}

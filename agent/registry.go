package agent

import "sync"

// Agent type identifiers. These are the values stored in the database
// and set as the WARDEN_AGENT_TYPE container env var.
const (
	// ClaudeCode identifies Anthropic's Claude Code CLI.
	ClaudeCode = "claude-code"
	// Codex identifies OpenAI's Codex CLI.
	Codex = "codex"

	// DefaultAgentType is the agent type used when none is specified.
	DefaultAgentType = ClaudeCode
)

// AllTypes lists all supported agent type identifiers in display order.
var AllTypes = []string{ClaudeCode, Codex}

// DisplayLabels maps agent type identifiers to human-readable labels.
var DisplayLabels = map[string]string{
	ClaudeCode: "Claude Code",
	Codex:      "OpenAI Codex",
}

// ShortLabel returns a compact display label for the given agent type,
// falling back to the type string itself when no mapping exists.
func ShortLabel(agentType string) string {
	switch agentType {
	case ClaudeCode:
		return "claude"
	case Codex:
		return "codex"
	default:
		return agentType
	}
}

// Registry holds StatusProvider instances keyed by agent type name.
// It allows the engine to resolve the correct provider for a container
// based on its WARDEN_AGENT_TYPE env var.
type Registry struct {
	mu        sync.RWMutex
	providers map[string]StatusProvider
}

// NewRegistry creates an empty agent registry.
func NewRegistry() *Registry {
	return &Registry{
		providers: make(map[string]StatusProvider),
	}
}

// Register adds a StatusProvider for the given agent type name.
func (r *Registry) Register(name string, provider StatusProvider) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.providers[name] = provider
}

// Get returns the StatusProvider for the given agent type name.
// Returns nil and false if no provider is registered for that name.
func (r *Registry) Get(name string) (StatusProvider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.providers[name]
	return p, ok
}

// Default returns the StatusProvider for the default agent type (claude-code).
// Returns nil if the default provider is not registered.
func (r *Registry) Default() StatusProvider {
	p, _ := r.Get(DefaultAgentType)
	return p
}

// Resolve returns the StatusProvider for the given agent type, falling
// back to the default provider if the name is empty or unregistered.
func (r *Registry) Resolve(agentType string) StatusProvider {
	if agentType == "" {
		return r.Default()
	}
	if p, ok := r.Get(agentType); ok {
		return p
	}
	return r.Default()
}

// Package db provides Warden's central persistence layer backed by SQLite.
//
// It stores three kinds of data:
//   - **Projects**: container configuration (name, image, mounts, env vars, etc.)
//   - **Settings**: key-value pairs (runtime, auditLogMode, disconnectKey)
//   - **Events**: centralized audit log from agent hooks, backend, frontend, containers
//
// The database lives at ~/.config/warden/warden.db (platform-dependent).
// All methods on [Store] are safe for concurrent use.
package db

import (
	"encoding/json"
	"time"
)

// QueryFilters controls which entries are returned by [Store.Query].
//
// Zero-value fields are ignored (no filtering). Limit defaults to
// [DefaultQueryLimit] when zero.
type QueryFilters struct {
	// Source restricts results to a single source layer.
	Source Source
	// Level restricts results to a single severity level.
	Level Level
	// ProjectID restricts results to a single project by its deterministic ID.
	ProjectID string
	// Worktree restricts results to a single worktree ID.
	Worktree string
	// Event restricts results to a single event type identifier.
	Event string
	// Events restricts results to any of the listed event types (OR).
	Events []string
	// ExcludeEvents excludes entries whose event type is in this list (NOT IN).
	// Mutually exclusive with Events; if both are set, Events takes precedence.
	ExcludeEvents []string
	// Since returns only entries with a timestamp at or after this time.
	Since time.Time
	// Until returns only entries with a timestamp strictly before this time.
	Until time.Time
	// Limit caps the number of returned entries. Defaults to DefaultQueryLimit if zero.
	Limit int
	// Offset skips this many entries before returning results.
	Offset int
}

// DefaultQueryLimit is applied when QueryFilters.Limit is zero.
const DefaultQueryLimit = 10_000

// defaultAgentType is the Go-side default matching the SQL schema default.
const defaultAgentType = "claude-code"

// Source identifies where a log entry originated.
type Source string

const (
	// SourceAgent is for Claude Code hook events (attention, session lifecycle, etc.).
	SourceAgent Source = "agent"
	// SourceBackend is for Go application events (slog-captured).
	SourceBackend Source = "backend"
	// SourceFrontend is for browser-side events posted via the API.
	SourceFrontend Source = "frontend"
	// SourceContainer is for container lifecycle events (create, stop, restart, etc.).
	SourceContainer Source = "container"
)

// Level indicates the severity of a log entry.
type Level string

const (
	// LevelInfo is the default severity for informational events.
	LevelInfo Level = "info"
	// LevelWarn indicates a warning condition.
	LevelWarn Level = "warn"
	// LevelError indicates an error condition.
	LevelError Level = "error"
)

// Entry is a single event log record.
//
// Source, Level, and Event are required.
type Entry struct {
	// Timestamp is when the event occurred (ISO 8601 with milliseconds).
	Timestamp time.Time `json:"ts"`
	// Source identifies the origin layer (agent, backend, frontend, container).
	Source Source `json:"source"`
	// Level is the severity of the entry (info, warn, error).
	Level Level `json:"level"`
	// Event is a snake_case identifier for the event type (e.g. "session_start").
	Event string `json:"event"`
	// ProjectID is the deterministic project identifier (sha256 of host path, 12 hex chars).
	ProjectID string `json:"projectId,omitempty"`
	// AgentType identifies the agent that produced this event (e.g. "claude-code", "codex").
	AgentType string `json:"agentType,omitempty"`
	// ContainerName is a snapshot of the container name at the time of the event.
	ContainerName string `json:"containerName,omitempty"`
	// Worktree is the worktree ID (only for agent events).
	Worktree string `json:"worktree,omitempty"`
	// Message is a human-readable description.
	Message string `json:"msg,omitempty"`
	// Data carries the raw event payload (for agent events, preserves hook JSON).
	Data json.RawMessage `json:"data,omitempty"`
	// Attrs carries structured key-value metadata.
	Attrs map[string]any `json:"attrs,omitempty"`
	// Category is the audit category (session, agent, prompt, config, system).
	// Computed at query time from the event name — not stored in the DB.
	Category string `json:"category,omitempty"`
	// SourceID is a content hash for deduplication of JSONL-sourced events.
	// When set, the DB uses INSERT OR IGNORE to silently drop duplicates.
	// Empty for hook and backend events (no dedup needed).
	SourceID string `json:"-"`
}

// DisplayProject returns the best available project label for human display.
// Prefers the container name snapshot (human-readable), falls back to ProjectID (hex hash).
func (e Entry) DisplayProject() string {
	if e.ContainerName != "" {
		return e.ContainerName
	}
	return e.ProjectID
}

// ProjectRow represents a project stored in the database.
//
// Complex fields (EnvVars, Mounts, OriginalMounts) are stored as opaque JSON.
// The service layer handles marshaling to/from engine types.
type ProjectRow struct {
	// ProjectID is the deterministic identifier (sha256 of host path, 12 hex chars).
	ProjectID string
	// Name is the user-chosen display label / Docker container name.
	Name string
	// HostPath is the absolute host directory mounted into the container.
	HostPath string
	// AddedAt is when the project was added to Warden.
	AddedAt time.Time
	// Image is the container image name.
	Image string
	// EnvVars is JSON-encoded map[string]string of user-provided env vars.
	EnvVars json.RawMessage
	// Mounts is JSON-encoded []Mount of additional bind mounts.
	Mounts json.RawMessage
	// OriginalMounts is JSON-encoded []Mount of pre-symlink-resolution mounts.
	OriginalMounts json.RawMessage
	// SkipPermissions controls whether terminals skip permission prompts.
	SkipPermissions bool
	// NetworkMode is the container's network isolation level (full/restricted/none).
	NetworkMode string
	// AllowedDomains is comma-separated domains for restricted mode.
	AllowedDomains string
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64
	// EnabledAccessItems is a comma-separated list of enabled access item IDs (e.g. "git,ssh").
	EnabledAccessItems string
	// AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").
	AgentType string
	// ContainerID is the Docker-assigned container ID (empty when no container exists).
	ContainerID string
	// ContainerName is the Docker container name (may differ from Name if renamed).
	ContainerName string
}

// ProjectAgentKey uniquely identifies a project+agent pair. Used as a map key
// where the compound (project_id, agent_type) identity is needed.
type ProjectAgentKey struct {
	ProjectID string
	AgentType string
}

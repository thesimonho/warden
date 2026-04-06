// Package api defines the request, response, and result types for the
// Warden service API. These types form the contract between the service
// layer, HTTP handlers, Go client, and TUI — consumers import this
// package for types without depending on the service implementation.
package api

import (
	"encoding/json"
	"time"

	"github.com/thesimonho/warden/constants"
)

// ProjectResponse is a project returned by the HTTP API. It mirrors the
// fields of engine.Project with explicit field declarations so the JSON
// contract is decoupled from the internal domain type.
type ProjectResponse struct {
	// ProjectID is the deterministic project identifier (sha256 of host path, 12 hex chars).
	ProjectID string `json:"projectId"`
	// ID is the Docker container ID (empty when no container exists).
	ID string `json:"id"`
	// Name is the user-chosen display label / Docker container name.
	Name string `json:"name"`
	// HostPath is the absolute host directory mounted into the container.
	HostPath string `json:"hostPath,omitempty"`
	// HasContainer is true when a Docker container is associated with this project.
	HasContainer bool   `json:"hasContainer"`
	Type         string `json:"type"`
	Image        string `json:"image"`
	OS           string `json:"os"`
	CreatedAt    int64  `json:"createdAt"`
	SSHPort      string `json:"sshPort"`
	// State is the Docker container state ("running", "exited", "not-found", etc).
	State string `json:"state"`
	// Status is the Docker container status string (e.g. "Up 2 hours").
	Status string `json:"status"`
	// AgentStatus is the agent activity state ("idle", "working", "unknown").
	AgentStatus string `json:"agentStatus"`
	// NeedsInput is true when any worktree requires user attention.
	NeedsInput bool `json:"needsInput,omitempty"`
	// NotificationType indicates why the agent needs attention
	// (e.g. "permission_prompt", "idle_prompt", "elicitation_dialog").
	NotificationType string `json:"notificationType,omitempty"`
	// ActiveWorktreeCount is the number of worktrees with connected terminals.
	ActiveWorktreeCount int `json:"activeWorktreeCount"`
	// TotalCost is the aggregate cost across all worktrees in USD.
	TotalCost float64 `json:"totalCost"`
	// IsEstimatedCost is true when the cost is an estimate (e.g. subscription users).
	IsEstimatedCost bool `json:"isEstimatedCost,omitempty"`
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64 `json:"costBudget"`
	// IsGitRepo indicates whether the container's /project is a git repository.
	IsGitRepo bool `json:"isGitRepo"`
	// AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").
	AgentType constants.AgentType `json:"agentType"`
	// SkipPermissions indicates whether terminals should skip permission prompts.
	SkipPermissions bool `json:"skipPermissions"`
	// MountedDir is the host directory mounted into the container.
	MountedDir string `json:"mountedDir,omitempty"`
	// WorkspaceDir is the container-side workspace directory (mount destination).
	WorkspaceDir string `json:"workspaceDir,omitempty"`
	// NetworkMode controls the container's network isolation level.
	NetworkMode NetworkMode `json:"networkMode"`
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// AgentVersion is the pinned CLI version installed in this container.
	AgentVersion string `json:"agentVersion,omitempty"`
}

// AddProjectRequest is the JSON body for registering a project directory.
// When Container is non-nil, a container is created atomically after the
// project is registered. If container creation fails, the project is
// cleaned up and the error is returned.
type AddProjectRequest struct {
	// Name is an optional container name override.
	Name string `json:"name,omitempty"`
	// ProjectPath is the absolute host directory to register as a project.
	ProjectPath string `json:"projectPath"`
	// AgentType selects the CLI agent to run (e.g. "claude-code", "codex").
	// Defaults to "claude-code" if omitted.
	AgentType string `json:"agentType,omitempty"`
	// Container holds optional container configuration. When provided, a
	// container is created as part of the same request.
	Container *CreateContainerRequest `json:"container,omitempty"`
}

// AddProjectResponse is the result of POST /api/v1/projects.
type AddProjectResponse struct {
	// Project holds the registered project result.
	Project ProjectResult `json:"project"`
	// Container holds the container result when a container was created.
	// Nil when the request did not include container configuration.
	Container *ContainerResult `json:"container,omitempty"`
}

// CreateWorktreeRequest is the JSON body for creating a new git worktree.
type CreateWorktreeRequest struct {
	// Name is the worktree name (must be a valid git branch name).
	Name string `json:"name"`
}

// WorktreeInputRequest is the JSON body for sending text to a worktree's terminal.
type WorktreeInputRequest struct {
	// Text is the input to send. Required, max 64KB.
	Text string `json:"text"`
	// PressEnter appends Enter after the text. Defaults to true if omitted.
	PressEnter *bool `json:"pressEnter,omitempty"`
}

// ShouldPressEnter returns whether Enter should be sent after the text.
func (r WorktreeInputRequest) ShouldPressEnter() bool {
	return r.PressEnter == nil || *r.PressEnter
}

// ProjectResult is the outcome of a project mutation (create, remove, stop, restart).
type ProjectResult struct {
	// ProjectID is the deterministic project identifier.
	ProjectID string `json:"projectId"`
	// AgentType is the agent type for this project (e.g. "claude-code", "codex").
	AgentType string `json:"agentType"`
	// Name is the user-chosen project display name.
	Name string `json:"name"`
	// ContainerID is the Docker container ID, when available.
	ContainerID string `json:"containerId,omitempty"`
}

// WorktreeResult is the outcome of a worktree mutation (create, connect,
// disconnect, kill, remove).
type WorktreeResult struct {
	// WorktreeID is the worktree identifier.
	WorktreeID string `json:"worktreeId"`
	// ProjectID is the deterministic project identifier the worktree belongs to.
	ProjectID string `json:"projectId"`
	// State is the worktree's terminal state after the mutation
	// ("connected", "shell", "background", "stopped"). Best-effort — may be
	// empty if the state could not be determined (e.g. container not running).
	State string `json:"state,omitempty"`
}

// ContainerResult holds the output of a container create, update, or delete
// operation. ContainerID is the Docker container ID. Name is the container
// name. For delete operations, these reflect the container that was removed.
type ContainerResult struct {
	// ContainerID is the Docker container ID.
	ContainerID string `json:"containerId"`
	// Name is the container name.
	Name string `json:"name"`
	// ProjectID is the deterministic project identifier.
	ProjectID string `json:"projectId"`
	// AgentType is the agent type for this container.
	AgentType string `json:"agentType"`
	// Recreated is true when the container was fully recreated (not just settings updated).
	Recreated bool `json:"recreated,omitempty"`
}

// ValidateContainerResult holds the output of infrastructure validation.
type ValidateContainerResult struct {
	Valid   bool
	Missing []string
}

// ProjectCostsResponse holds session-level cost data for a project.
type ProjectCostsResponse struct {
	ProjectID   string             `json:"projectId"`
	AgentType   string             `json:"agentType"`
	TotalCost   float64            `json:"totalCost"`
	IsEstimated bool               `json:"isEstimated"`
	Sessions    []SessionCostEntry `json:"sessions"`
}

// SessionCostEntry holds cost data for a single agent session.
type SessionCostEntry struct {
	SessionID   string  `json:"sessionId"`
	Cost        float64 `json:"cost"`
	IsEstimated bool    `json:"isEstimated"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

// BudgetStatusResponse holds the budget state for a project.
type BudgetStatusResponse struct {
	ProjectID       string  `json:"projectId"`
	AgentType       string  `json:"agentType"`
	EffectiveBudget float64 `json:"effectiveBudget"`
	TotalCost       float64 `json:"totalCost"`
	IsOverBudget    bool    `json:"isOverBudget"`
	IsEstimatedCost bool    `json:"isEstimatedCost"`
	// BudgetSource is "project" (per-project), "global" (default), or "none" (no budget set).
	BudgetSource string `json:"budgetSource"`
}

// BudgetSource identifies where a project's effective budget comes from.
type BudgetSource string

const (
	// BudgetSourceProject means the project has a per-project budget set.
	BudgetSourceProject BudgetSource = "project"
	// BudgetSourceGlobal means the project uses the global default budget.
	BudgetSourceGlobal BudgetSource = "global"
	// BudgetSourceNone means no budget is configured.
	BudgetSourceNone BudgetSource = "none"
)

// AuditLogMode controls which events are written to the database.
type AuditLogMode string

const (
	// AuditLogOff disables all audit logging. Nothing is written.
	AuditLogOff AuditLogMode = "off"
	// AuditLogStandard logs session lifecycle, worktree lifecycle,
	// and system events.
	AuditLogStandard AuditLogMode = "standard"
	// AuditLogDetailed logs everything in standard plus tool use, user
	// prompts, permissions, subagents, config changes, and debug events.
	AuditLogDetailed AuditLogMode = "detailed"
)

// SettingsResponse holds the current server-side settings.
type SettingsResponse struct {
	Runtime              string       `json:"runtime"`
	AuditLogMode         AuditLogMode `json:"auditLogMode"`
	DisconnectKey        string       `json:"disconnectKey"`
	DefaultProjectBudget float64      `json:"defaultProjectBudget"`

	// Budget enforcement actions — what happens when a project exceeds its budget.
	BudgetActionWarn          bool `json:"budgetActionWarn"`
	BudgetActionStopWorktrees bool `json:"budgetActionStopWorktrees"`
	BudgetActionStopContainer bool `json:"budgetActionStopContainer"`
	BudgetActionPreventStart  bool `json:"budgetActionPreventStart"`

	// WorkingDirectory is the server process's working directory. Used by
	// development tooling to auto-create projects without manual path entry.
	WorkingDirectory string `json:"workingDirectory"`

	// Version is the server build version (e.g. "v0.5.2", "dev").
	Version string `json:"version"`

	// Pinned CLI versions installed in containers.
	ClaudeCodeVersion string `json:"claudeCodeVersion"`
	CodexVersion      string `json:"codexVersion"`
}

// UpdateSettingsRequest holds the fields that can be updated.
// Pointer fields allow distinguishing "not provided" from zero values.
type UpdateSettingsRequest struct {
	AuditLogMode         *AuditLogMode `json:"auditLogMode"`
	DisconnectKey        *string       `json:"disconnectKey"`
	DefaultProjectBudget *float64      `json:"defaultProjectBudget"`

	BudgetActionWarn          *bool `json:"budgetActionWarn"`
	BudgetActionStopWorktrees *bool `json:"budgetActionStopWorktrees"`
	BudgetActionStopContainer *bool `json:"budgetActionStopContainer"`
	BudgetActionPreventStart  *bool `json:"budgetActionPreventStart"`
}

// UpdateSettingsResult holds the output of a settings update.
type UpdateSettingsResult struct {
	RestartRequired bool
}

// PostAuditEventRequest holds the fields for writing a custom audit event.
type PostAuditEventRequest struct {
	// Event is a snake_case identifier for the event type (e.g. "deployment_started"). Required.
	Event string `json:"event"`
	// Source identifies the origin of the event. Must be a valid AuditSource value.
	// Defaults to "external" if omitted.
	Source string `json:"source,omitempty"`
	// Level is the severity ("info", "warn", "error"). Defaults to "info" if omitted.
	Level string `json:"level,omitempty"`
	// Message is a human-readable description.
	Message string `json:"message,omitempty"`
	// ProjectID associates the event with a project. Optional.
	ProjectID string `json:"projectId,omitempty"`
	// AgentType scopes the event to an agent type (e.g. "claude-code", "codex"). Optional.
	AgentType string `json:"agentType,omitempty"`
	// Worktree associates the event with a worktree. Optional.
	Worktree string `json:"worktree,omitempty"`
	// Data carries a raw JSON payload for structured event data.
	Data json.RawMessage `json:"data,omitempty"`
	// Attrs carries key-value metadata.
	Attrs map[string]any `json:"attrs,omitempty"`
}

// DefaultMount represents a resolved default bind mount for the
// create container form.
type DefaultMount struct {
	HostPath      string `json:"hostPath"`
	ContainerPath string `json:"containerPath"`
	ReadOnly      bool   `json:"readOnly"`
	// AgentType restricts this mount to a specific agent type.
	// Empty means the mount applies to all agent types.
	AgentType string `json:"agentType,omitempty"`
	// Required marks this mount as mandatory for the agent to function.
	// Clients must not allow users to remove or change the container path of required mounts.
	Required bool `json:"required,omitempty"`
}

// DefaultEnvVar represents an auto-detected environment variable for the
// create container form.
type DefaultEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// RuntimeDefault describes a runtime for the create container form,
// including auto-detection results and the domains/env vars it contributes.
type RuntimeDefault struct {
	// ID is the unique identifier (e.g. "node", "python", "go").
	ID string `json:"id"`
	// Label is the human-readable name (e.g. "Node.js", "Python").
	Label string `json:"label"`
	// Description briefly explains what gets installed.
	Description string `json:"description"`
	// AlwaysEnabled means this runtime cannot be deselected.
	AlwaysEnabled bool `json:"alwaysEnabled"`
	// Detected is true when marker files were found in the project directory.
	Detected bool `json:"detected"`
	// Domains lists network domains required for this runtime's package registry.
	Domains []string `json:"domains"`
	// EnvVars maps environment variable names to values set when enabled.
	EnvVars map[string]string `json:"envVars"`
}

// DefaultsResponse holds server-resolved default values for the
// create container form.
type DefaultsResponse struct {
	HomeDir           string              `json:"homeDir"`
	ContainerHomeDir  string              `json:"containerHomeDir"`
	Mounts            []DefaultMount      `json:"mounts,omitempty"`
	EnvVars           []DefaultEnvVar     `json:"envVars,omitempty"`
	RestrictedDomains map[string][]string `json:"restrictedDomains,omitempty"`
	// Runtimes lists available language runtimes with detection results.
	Runtimes []RuntimeDefault `json:"runtimes,omitempty"`
	// Template holds project template values loaded from .warden.json, if present.
	Template *ProjectTemplate `json:"template,omitempty"`
}

// ProjectTemplate holds configuration values from a .warden.json file.
// All fields are optional — only set fields override defaults in the form.
//
// Excluded fields (security): envVars (may contain secrets/tokens) and
// accessItems (resolve to credentials). These are never read from or
// written to .warden.json.
type ProjectTemplate struct {
	Image           string                           `json:"image,omitempty"`
	SkipPermissions *bool                            `json:"skipPermissions,omitempty"`
	NetworkMode     NetworkMode                      `json:"networkMode,omitempty"`
	CostBudget      *float64                         `json:"costBudget,omitempty"`
	Runtimes        []string                         `json:"runtimes,omitempty"`
	Agents          map[string]AgentTemplateOverride `json:"agents,omitempty"`
}

// AgentTemplateOverride holds agent-type-specific overrides within a project template.
type AgentTemplateOverride struct {
	AllowedDomains []string `json:"allowedDomains,omitempty"`
}

// DirEntry represents a filesystem entry in the browser.
type DirEntry struct {
	Name  string `json:"name"`
	Path  string `json:"path"`
	IsDir bool   `json:"isDir"`
}

// --- Diff ---

// DiffFileSummary describes a single file's change statistics in a worktree diff.
type DiffFileSummary struct {
	// Path is the file path relative to the worktree root.
	Path string `json:"path"`
	// OldPath is the previous path for renamed files.
	OldPath string `json:"oldPath,omitempty"`
	// Additions is the number of lines added.
	Additions int `json:"additions"`
	// Deletions is the number of lines removed.
	Deletions int `json:"deletions"`
	// IsBinary is true when the file is a binary file.
	IsBinary bool `json:"isBinary"`
	// Status is the change type: "added", "modified", "deleted", or "renamed".
	Status string `json:"status"`
}

// DiffResponse holds the complete diff output for a worktree.
type DiffResponse struct {
	// Files lists per-file change statistics.
	Files []DiffFileSummary `json:"files"`
	// RawDiff is the unified diff output from git.
	RawDiff string `json:"rawDiff"`
	// TotalAdditions is the sum of additions across all files.
	TotalAdditions int `json:"totalAdditions"`
	// TotalDeletions is the sum of deletions across all files.
	TotalDeletions int `json:"totalDeletions"`
	// Truncated is true when the raw diff exceeded the size limit and was capped.
	Truncated bool `json:"truncated"`
}

// --- Audit Log ---

// AuditSource identifies where a log entry originated.
type AuditSource string

const (
	// AuditSourceAgent is for agent hook events (attention, session lifecycle, etc.).
	AuditSourceAgent AuditSource = "agent"
	// AuditSourceBackend is for Go application events (slog-captured).
	AuditSourceBackend AuditSource = "backend"
	// AuditSourceFrontend is for browser-side events posted via the API.
	AuditSourceFrontend AuditSource = "frontend"
	// AuditSourceContainer is for container lifecycle events (create, stop, restart, etc.).
	AuditSourceContainer AuditSource = "container"
	// AuditSourceExternal is for events posted by external integrators via the API.
	AuditSourceExternal AuditSource = "external"
)

// IsValidAuditSource reports whether the given string is a recognized audit source.
func IsValidAuditSource(s string) bool {
	switch AuditSource(s) {
	case AuditSourceAgent, AuditSourceBackend, AuditSourceFrontend, AuditSourceContainer, AuditSourceExternal:
		return true
	default:
		return false
	}
}

// AuditLevel indicates the severity of a log entry.
type AuditLevel string

const (
	// AuditLevelInfo is the default severity for informational events.
	AuditLevelInfo AuditLevel = "info"
	// AuditLevelWarn indicates a warning condition.
	AuditLevelWarn AuditLevel = "warn"
	// AuditLevelError indicates an error condition.
	AuditLevelError AuditLevel = "error"
)

// AuditEntry is a single event log record.
//
// Source, Level, and Event are required.
type AuditEntry struct {
	// ID is the database row identifier. Unique across all entries.
	ID int64 `json:"id"`
	// Timestamp is when the event occurred (ISO 8601 with milliseconds).
	Timestamp time.Time `json:"ts"`
	// Source identifies the origin layer (agent, backend, frontend, container).
	Source AuditSource `json:"source"`
	// Level is the severity of the entry (info, warn, error).
	Level AuditLevel `json:"level"`
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
func (e AuditEntry) DisplayProject() string {
	if e.ContainerName != "" {
		return e.ContainerName
	}
	return e.ProjectID
}

// AuditCategory groups audit events by purpose.
type AuditCategory string

const (
	// AuditCategorySession groups session lifecycle events (start, end, stop, exit).
	AuditCategorySession AuditCategory = "session"
	// AuditCategoryAgent groups agent activity: tool use, permissions, subagents,
	// tasks, and MCP elicitation.
	AuditCategoryAgent AuditCategory = "agent"
	// AuditCategoryPrompt groups user prompt events.
	AuditCategoryPrompt AuditCategory = "prompt"
	// AuditCategoryConfig groups configuration changes and instruction loading.
	AuditCategoryConfig AuditCategory = "config"
	// AuditCategoryBudget groups cost budget enforcement events
	// (exceeded, worktrees stopped, container stopped, enforcement failures).
	AuditCategoryBudget AuditCategory = "budget"
	// AuditCategorySystem groups process management and backend operational events.
	AuditCategorySystem AuditCategory = "system"
	// AuditCategoryDebug groups auto-captured slog backend events and any
	// events not explicitly mapped to another category.
	AuditCategoryDebug AuditCategory = "debug"
)

// StandardAuditCategories defines which categories are logged in standard
// mode. Categories not in this list are only logged in detailed mode.
// This is the single source of truth — db.AuditWriter derives its
// standard events allowlist from this via service.StandardAuditEvents().
var StandardAuditCategories = []AuditCategory{
	AuditCategorySession,
	AuditCategoryBudget,
	AuditCategorySystem,
}

// AuditFilters controls which audit entries are returned.
type AuditFilters struct {
	// ProjectID restricts results to a single project by its deterministic ID.
	ProjectID string
	// Worktree restricts results to a single worktree ID.
	Worktree string
	// Category restricts results to a single audit category.
	Category AuditCategory
	// Source restricts results to a single source layer (agent, backend, frontend, container).
	Source string
	// Level restricts results to a single severity level (info, warn, error).
	Level string
	// Since returns only entries at or after this time (RFC3339).
	Since string
	// Until returns only entries before this time (RFC3339).
	Until string
	// Limit caps the number of returned entries.
	Limit int
	// Offset skips this many entries before returning results.
	Offset int
}

// AuditSummary provides aggregate statistics for the audit log.
type AuditSummary struct {
	// TotalSessions is the number of session_start events.
	TotalSessions int `json:"totalSessions"`
	// TotalToolUses is the number of tool_use events.
	TotalToolUses int `json:"totalToolUses"`
	// TotalPrompts is the number of user_prompt events.
	TotalPrompts int `json:"totalPrompts"`
	// TotalCostUSD is the aggregate cost across all projects.
	TotalCostUSD float64 `json:"totalCostUsd"`
	// UniqueProjects is the number of distinct projects with events.
	UniqueProjects int `json:"uniqueProjects"`
	// UniqueWorktrees is the number of distinct worktrees with events.
	UniqueWorktrees int `json:"uniqueWorktrees"`
	// TopTools lists the most frequently used tools with counts.
	TopTools []ToolCount `json:"topTools"`
	// TimeRange holds the earliest and latest event timestamps.
	TimeRange TimeRange `json:"timeRange"`
}

// ToolCount pairs a tool name with its invocation count.
type ToolCount struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// TimeRange holds the earliest and latest timestamps in the result.
type TimeRange struct {
	Earliest string `json:"earliest,omitempty"`
	Latest   string `json:"latest,omitempty"`
}

// --- Clipboard ---

// ClipboardUploadResponse is the result of uploading a file to a container's
// clipboard staging directory for the xclip shim to serve.
type ClipboardUploadResponse struct {
	// Path is the absolute path of the staged file inside the container.
	Path string `json:"path"`
}

// --- Network ---

// NetworkMode controls the container's network isolation level.
type NetworkMode string

const (
	// NetworkModeFull allows unrestricted internet access (default).
	NetworkModeFull NetworkMode = "full"
	// NetworkModeRestricted allows access only to explicitly allowed domains.
	NetworkModeRestricted NetworkMode = "restricted"
	// NetworkModeNone blocks all outbound network access (air-gapped).
	NetworkModeNone NetworkMode = "none"
)

// IsValidNetworkMode reports whether the given string is a valid network mode.
func IsValidNetworkMode(mode string) bool {
	switch NetworkMode(mode) {
	case NetworkModeFull, NetworkModeRestricted, NetworkModeNone:
		return true
	default:
		return false
	}
}

// --- Container ---

// Mount describes a bind mount from the host into the container.
type Mount struct {
	// HostPath is the absolute path on the host.
	HostPath string `json:"hostPath"`
	// ContainerPath is the absolute path inside the container.
	ContainerPath string `json:"containerPath"`
	// ReadOnly mounts the path as read-only inside the container.
	ReadOnly bool `json:"readOnly"`
}

// CreateContainerRequest is the JSON body for creating a new project container.
type CreateContainerRequest struct {
	Name        string `json:"name"`
	Image       string `json:"image"`
	ProjectPath string `json:"projectPath"`
	// AgentType selects the CLI agent to run (e.g. "claude-code", "codex"). Defaults to "claude-code".
	AgentType constants.AgentType `json:"agentType,omitempty"`
	EnvVars   map[string]string   `json:"envVars,omitempty"`
	// Mounts is a list of additional bind mounts from host into the container.
	Mounts []Mount `json:"mounts,omitempty"`
	// SkipPermissions controls whether terminals skip permission prompts.
	// Stored as a Docker label on the container.
	SkipPermissions bool `json:"skipPermissions,omitempty"`
	// NetworkMode controls the container's network isolation level.
	NetworkMode NetworkMode `json:"networkMode,omitempty"`
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64 `json:"costBudget,omitempty"`
	// EnabledAccessItems lists active access item IDs (e.g. ["git","ssh"]).
	EnabledAccessItems []string `json:"enabledAccessItems,omitempty"`
	// EnabledRuntimes lists active runtime IDs (e.g. ["node","python","go"]).
	EnabledRuntimes []string `json:"enabledRuntimes,omitempty"`
}

// ContainerConfig holds the editable configuration of an existing container.
// Returned by InspectContainer for populating the edit form.
type ContainerConfig struct {
	Name        string `json:"name"`
	Image       string `json:"image"`
	ProjectPath string `json:"projectPath"`
	// AgentType identifies the CLI agent running in this project.
	AgentType       constants.AgentType `json:"agentType"`
	EnvVars         map[string]string   `json:"envVars,omitempty"`
	Mounts          []Mount             `json:"mounts,omitempty"`
	SkipPermissions bool                `json:"skipPermissions"`
	// NetworkMode controls the container's network isolation level.
	NetworkMode NetworkMode `json:"networkMode"`
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64 `json:"costBudget"`
	// EnabledAccessItems lists active access item IDs (e.g. ["git","ssh"]).
	EnabledAccessItems []string `json:"enabledAccessItems,omitempty"`
	// EnabledRuntimes lists active runtime IDs (e.g. ["node","python","go"]).
	EnabledRuntimes []string `json:"enabledRuntimes,omitempty"`
}

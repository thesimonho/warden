// Package api defines the request, response, and result types for the
// Warden service API. These types form the contract between the service
// layer, HTTP handlers, Go client, and TUI — consumers import this
// package for types without depending on the service implementation.
package api

// ProjectResult is the outcome of a project mutation (create, remove, stop,
// restart). ProjectID is always populated. ContainerID is populated when the
// operation targets a specific container.
type ProjectResult struct {
	// ProjectID is the deterministic project identifier.
	ProjectID string `json:"projectId"`
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
}

// ValidateContainerResult holds the output of infrastructure validation.
type ValidateContainerResult struct {
	Valid   bool
	Missing []string
}

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
}

// UpdateSettingsRequest holds the fields that can be updated.
// Pointer fields allow distinguishing "not provided" from zero values.
type UpdateSettingsRequest struct {
	Runtime              *string       `json:"runtime"`
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

// PostAuditEventRequest holds the fields for writing a frontend-posted audit event.
type PostAuditEventRequest struct {
	Event   string         `json:"event"`
	Level   string         `json:"level"`
	Message string         `json:"message"`
	Attrs   map[string]any `json:"attrs,omitempty"`
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
}

// DefaultEnvVar represents an auto-detected environment variable for the
// create container form.
type DefaultEnvVar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// DefaultsResponse holds server-resolved default values for the
// create container form.
type DefaultsResponse struct {
	HomeDir          string          `json:"homeDir"`
	ContainerHomeDir string          `json:"containerHomeDir"`
	Mounts           []DefaultMount  `json:"mounts,omitempty"`
	EnvVars          []DefaultEnvVar `json:"envVars,omitempty"`
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

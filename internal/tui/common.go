package tui

import (
	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
)

// Tab identifies the top-level navigation tabs.
type Tab int

const (
	// TabProjects is the project list view (home screen).
	TabProjects Tab = iota
	// TabSettings is the settings view.
	TabSettings
	// TabAccess is the access management view.
	TabAccess
	// TabAudit is the unified audit log viewer.
	TabAudit
)

// TabLabels maps each tab to its display label.
var TabLabels = map[Tab]string{
	TabProjects: "Projects",
	TabSettings: "Settings",
	TabAudit:    "Audit Log",
	TabAccess:   "Access",
}

// --- Async result messages ---
// These are returned by tea.Cmd functions when data arrives or
// operations complete. Each view handles the messages it cares about.

// ProjectsLoadedMsg carries the result of a ListProjects call.
type ProjectsLoadedMsg struct {
	Projects []engine.Project
	Err      error
}

// WorktreesLoadedMsg carries the result of a ListWorktrees call.
type WorktreesLoadedMsg struct {
	Worktrees []engine.Worktree
	Err       error
}

// SettingsLoadedMsg carries the result of a GetSettings call.
type SettingsLoadedMsg struct {
	Settings *api.SettingsResponse
	Err      error
}

// AuditLogLoadedMsg carries the result of a GetAuditLog call.
type AuditLogLoadedMsg struct {
	Entries []api.AuditEntry
	Summary *api.AuditSummary
	Err     error
}

// AuditProjectsLoadedMsg carries the result of a GetAuditProjects call.
type AuditProjectsLoadedMsg struct {
	Names []string
	Err   error
}

// DefaultsLoadedMsg carries the result of a GetDefaults call.
type DefaultsLoadedMsg struct {
	Defaults *api.DefaultsResponse
	Err      error
}

// AccessItemsLoadedMsg carries the result of a ListAccessItems call.
type AccessItemsLoadedMsg struct {
	Items []api.AccessItemResponse
	Err   error
}

// AccessItemResolvedMsg carries the result of a ResolveAccessItems call.
type AccessItemResolvedMsg struct {
	Items []access.ResolvedItem
	Err   error
}

// --- Operation result messages ---

// OperationResultMsg carries the result of a mutating operation.
type OperationResultMsg struct {
	Operation string
	Err       error
}

// --- Navigation messages ---

// NavigateMsg requests a view transition.
type NavigateMsg struct {
	// Tab is the target tab.
	Tab Tab
	// ProjectID is set when navigating to project detail.
	ProjectID string
	// AgentType is the agent type for the project.
	AgentType string
	// ProjectName is the display name for the project.
	ProjectName string
}

// NavigateBackMsg requests returning to the previous view.
type NavigateBackMsg struct{}

// --- Real-time event messages ---

// SSEEventMsg wraps an SSE event received from the event bus.
type SSEEventMsg eventbus.SSEEvent

// EventStreamClosedMsg indicates the SSE channel was closed.
type EventStreamClosedMsg struct{}

// --- Terminal messages ---

// TerminalExitedMsg indicates the terminal passthrough ended.
type TerminalExitedMsg struct {
	Err error
}

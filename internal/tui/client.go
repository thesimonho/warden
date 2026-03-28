// Package tui implements a terminal user interface for Warden using
// Bubble Tea v2. It serves both as a usable product and as a reference
// implementation for Go developers consuming the client/ package.
package tui

import (
	"context"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/client"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/runtime"
)

// Client abstracts all Warden operations for the TUI. It is the key
// architectural boundary — satisfied by both ServiceAdapter (embedded
// mode) and client.Client (HTTP mode).
//
// Each method maps 1:1 to an API endpoint. Developers reading this
// interface can cross-reference with the API docs and the web
// dashboard's api.ts.
type Client interface {
	// ListProjects returns all configured projects with container state.
	// API: GET /api/v1/projects
	ListProjects(ctx context.Context) ([]engine.Project, error)

	// AddProject registers a project directory in Warden.
	// API: POST /api/v1/projects
	AddProject(ctx context.Context, name, hostPath string) (*api.ProjectResult, error)

	// RemoveProject removes a project by its project ID.
	// API: DELETE /api/v1/projects/{projectId}
	RemoveProject(ctx context.Context, projectID string) (*api.ProjectResult, error)

	// StopProject stops a running project's container.
	// API: POST /api/v1/projects/{projectId}/stop
	StopProject(ctx context.Context, projectID string) (*api.ProjectResult, error)

	// RestartProject restarts a project's container.
	// API: POST /api/v1/projects/{projectId}/restart
	RestartProject(ctx context.Context, projectID string) (*api.ProjectResult, error)

	// ListWorktrees returns all worktrees for a project with terminal state.
	// API: GET /api/v1/projects/{projectId}/worktrees
	ListWorktrees(ctx context.Context, projectID string) ([]engine.Worktree, error)

	// CreateWorktree creates a git worktree and connects a terminal.
	// API: POST /api/v1/projects/{projectId}/worktrees
	CreateWorktree(ctx context.Context, projectID, name string) (*api.WorktreeResult, error)

	// ConnectTerminal starts or reconnects a terminal for a worktree.
	// API: POST /api/v1/projects/{projectId}/worktrees/{wid}/connect
	ConnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error)

	// DisconnectTerminal closes the terminal viewer for a worktree.
	// API: POST /api/v1/projects/{projectId}/worktrees/{wid}/disconnect
	DisconnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error)

	// KillWorktreeProcess kills the abduco session and all child processes.
	// API: POST /api/v1/projects/{projectId}/worktrees/{wid}/kill
	KillWorktreeProcess(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error)

	// RemoveWorktree kills the process and removes the git worktree.
	// API: DELETE /api/v1/projects/{projectId}/worktrees/{wid}
	RemoveWorktree(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error)

	// CleanupWorktrees removes orphaned worktree directories.
	// API: POST /api/v1/projects/{projectId}/worktrees/cleanup
	CleanupWorktrees(ctx context.Context, projectID string) ([]string, error)

	// GetWorktreeDiff returns uncommitted changes for a worktree.
	// API: GET /api/v1/projects/{projectId}/worktrees/{wid}/diff
	GetWorktreeDiff(ctx context.Context, projectID, worktreeID string) (*api.DiffResponse, error)

	// CreateContainer creates a new container for the given project.
	// API: POST /api/v1/projects/{projectId}/container
	CreateContainer(ctx context.Context, projectID string, req engine.CreateContainerRequest) (*api.ContainerResult, error)

	// ResetProjectCosts removes all cost history for a project.
	// API: DELETE /api/v1/projects/{projectId}/costs
	ResetProjectCosts(ctx context.Context, projectID string) error

	// PurgeProjectAudit removes all audit events for a project.
	// API: DELETE /api/v1/projects/{projectId}/audit
	PurgeProjectAudit(ctx context.Context, projectID string) error

	// DeleteContainer stops and removes the container for the given project.
	// API: DELETE /api/v1/projects/{projectId}/container
	DeleteContainer(ctx context.Context, projectID string) (*api.ContainerResult, error)

	// InspectContainer returns the editable configuration of the project's container.
	// API: GET /api/v1/projects/{projectId}/container/config
	InspectContainer(ctx context.Context, projectID string) (*engine.ContainerConfig, error)

	// UpdateContainer recreates the project's container with updated configuration.
	// API: PUT /api/v1/projects/{projectId}/container
	UpdateContainer(ctx context.Context, projectID string, req engine.CreateContainerRequest) (*api.ContainerResult, error)

	// ValidateContainer checks whether the project's container has Warden infrastructure.
	// API: GET /api/v1/projects/{projectId}/container/validate
	ValidateContainer(ctx context.Context, projectID string) (*api.ValidateContainerResult, error)

	// GetSettings returns the current server-side settings.
	// API: GET /api/v1/settings
	GetSettings(ctx context.Context) (*api.SettingsResponse, error)

	// UpdateSettings applies setting changes.
	// API: PUT /api/v1/settings
	UpdateSettings(ctx context.Context, req api.UpdateSettingsRequest) (*api.UpdateSettingsResult, error)

	// GetDefaults returns server-resolved defaults for the create container form.
	// API: GET /api/v1/defaults
	GetDefaults(ctx context.Context) (*api.DefaultsResponse, error)

	// ListDirectories returns subdirectories at a path for the filesystem browser.
	// API: GET /api/v1/filesystem/directories?path=...
	ListDirectories(ctx context.Context, path string) ([]api.DirEntry, error)

	// ListRuntimes returns available container runtimes.
	// API: GET /api/v1/runtimes
	ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error)

	// GetAuditLog returns filtered audit events.
	// API: GET /api/v1/audit
	GetAuditLog(ctx context.Context, filters api.AuditFilters) ([]db.Entry, error)

	// GetAuditSummary returns aggregate audit statistics.
	// API: GET /api/v1/audit/summary
	GetAuditSummary(ctx context.Context, filters api.AuditFilters) (*api.AuditSummary, error)

	// GetAuditProjects returns distinct project names from the audit log.
	// API: GET /api/v1/audit/projects
	GetAuditProjects(ctx context.Context) ([]string, error)

	// PostAuditEvent writes a frontend event to the audit log.
	// API: POST /api/v1/audit
	PostAuditEvent(ctx context.Context, req api.PostAuditEventRequest) error

	// DeleteAuditEvents removes events matching the given filters.
	// API: DELETE /api/v1/audit
	DeleteAuditEvents(ctx context.Context, filters api.AuditFilters) error

	// SubscribeEvents returns a channel of real-time SSE events and an
	// unsubscribe function. The channel is closed when the context is
	// cancelled or the connection drops.
	// API: GET /api/v1/events (SSE)
	SubscribeEvents(ctx context.Context) (<-chan eventbus.SSEEvent, func(), error)

	// AttachTerminal returns a bidirectional connection to a worktree's
	// abduco session. In embedded mode this uses docker exec; in HTTP
	// mode this uses WebSocket to /api/v1/projects/{id}/ws/{wid}.
	AttachTerminal(ctx context.Context, projectID, worktreeID string) (client.TerminalConnection, error)
}

// Package tui implements a terminal user interface for Warden using
// Bubble Tea v2. It serves both as a usable product and as a reference
// implementation for Go developers consuming the client/ package.
package tui

import (
	"context"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/client"
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
	AddProject(ctx context.Context, name, hostPath, agentType string) (*api.ProjectResult, error)

	// RemoveProject removes a project by its project ID.
	// API: DELETE /api/v1/projects/{projectId}/{agentType}
	RemoveProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error)

	// StopProject stops a running project's container.
	// API: POST /api/v1/projects/{projectId}/{agentType}/stop
	StopProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error)

	// RestartProject restarts a project's container.
	// API: POST /api/v1/projects/{projectId}/{agentType}/restart
	RestartProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error)

	// ListWorktrees returns all worktrees for a project with terminal state.
	// API: GET /api/v1/projects/{projectId}/{agentType}/worktrees
	ListWorktrees(ctx context.Context, projectID, agentType string) ([]engine.Worktree, error)

	// CreateWorktree creates a git worktree and connects a terminal.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees
	CreateWorktree(ctx context.Context, projectID, agentType, name string) (*api.WorktreeResult, error)

	// ConnectTerminal starts or reconnects a terminal for a worktree.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect
	ConnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error)

	// DisconnectTerminal closes the terminal viewer for a worktree.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect
	DisconnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error)

	// KillWorktreeProcess kills the tmux session and all child processes.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill
	KillWorktreeProcess(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error)

	// ResetWorktree clears all history for a worktree without removing it.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/reset
	ResetWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error)

	// RemoveWorktree kills the process and removes the git worktree.
	// API: DELETE /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}
	RemoveWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error)

	// CleanupWorktrees removes orphaned worktree directories.
	// API: POST /api/v1/projects/{projectId}/{agentType}/worktrees/cleanup
	CleanupWorktrees(ctx context.Context, projectID, agentType string) ([]string, error)

	// GetWorktreeDiff returns uncommitted changes for a worktree.
	// API: GET /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff
	GetWorktreeDiff(ctx context.Context, projectID, agentType, worktreeID string) (*api.DiffResponse, error)

	// CreateContainer creates a new container for the given project.
	// API: POST /api/v1/projects/{projectId}/{agentType}/container
	CreateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*api.ContainerResult, error)

	// ResetProjectCosts removes all cost history for a project.
	// API: DELETE /api/v1/projects/{projectId}/{agentType}/costs
	ResetProjectCosts(ctx context.Context, projectID, agentType string) error

	// PurgeProjectAudit removes all audit events for a project.
	// API: DELETE /api/v1/projects/{projectId}/{agentType}/audit
	PurgeProjectAudit(ctx context.Context, projectID, agentType string) error

	// DeleteContainer stops and removes the container for the given project.
	// API: DELETE /api/v1/projects/{projectId}/{agentType}/container
	DeleteContainer(ctx context.Context, projectID, agentType string) (*api.ContainerResult, error)

	// InspectContainer returns the editable configuration of the project's container.
	// API: GET /api/v1/projects/{projectId}/{agentType}/container/config
	InspectContainer(ctx context.Context, projectID, agentType string) (*api.ContainerConfig, error)

	// UpdateContainer recreates the project's container with updated configuration.
	// API: PUT /api/v1/projects/{projectId}/{agentType}/container
	UpdateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*api.ContainerResult, error)

	// ValidateContainer checks whether the project's container has Warden infrastructure.
	// API: GET /api/v1/projects/{projectId}/{agentType}/container/validate
	ValidateContainer(ctx context.Context, projectID, agentType string) (*api.ValidateContainerResult, error)

	// GetSettings returns the current server-side settings.
	// API: GET /api/v1/settings
	GetSettings(ctx context.Context) (*api.SettingsResponse, error)

	// UpdateSettings applies setting changes.
	// API: PUT /api/v1/settings
	UpdateSettings(ctx context.Context, req api.UpdateSettingsRequest) (*api.UpdateSettingsResult, error)

	// GetDefaults returns server-resolved defaults for the create container form.
	// When projectPath is non-empty, runtime detection scans that directory.
	// API: GET /api/v1/defaults?path={projectPath}
	GetDefaults(ctx context.Context, projectPath string) (*api.DefaultsResponse, error)

	// ReadProjectTemplate reads a .warden.json template from an arbitrary path.
	// API: GET /api/v1/template?path={filePath}
	ReadProjectTemplate(ctx context.Context, filePath string) (*api.ProjectTemplate, error)

	// ListDirectories returns filesystem entries at a path for the browser.
	// When includeFiles is true, files are returned alongside directories.
	// API: GET /api/v1/filesystem/directories?path=...&mode=file
	ListDirectories(ctx context.Context, path string, includeFiles bool) ([]api.DirEntry, error)

	// ListRuntimes returns available container runtimes.
	// API: GET /api/v1/runtimes
	ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error)

	// GetAuditLog returns filtered audit events.
	// API: GET /api/v1/audit
	GetAuditLog(ctx context.Context, filters api.AuditFilters) ([]api.AuditEntry, error)

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

	// ListAccessItems returns all access items with detection status.
	// API: GET /api/v1/access
	ListAccessItems(ctx context.Context) (*api.AccessItemListResponse, error)

	// GetAccessItem returns a single access item by ID.
	// API: GET /api/v1/access/{id}
	GetAccessItem(ctx context.Context, id string) (*api.AccessItemResponse, error)

	// CreateAccessItem creates a user-defined access item.
	// API: POST /api/v1/access
	CreateAccessItem(ctx context.Context, req api.CreateAccessItemRequest) (*access.Item, error)

	// UpdateAccessItem updates a user-defined access item.
	// API: PUT /api/v1/access/{id}
	UpdateAccessItem(ctx context.Context, id string, req api.UpdateAccessItemRequest) (*access.Item, error)

	// DeleteAccessItem removes a user-defined access item.
	// API: DELETE /api/v1/access/{id}
	DeleteAccessItem(ctx context.Context, id string) error

	// ResetAccessItem restores a built-in access item to its default.
	// API: POST /api/v1/access/{id}/reset
	ResetAccessItem(ctx context.Context, id string) (*access.Item, error)

	// ResolveAccessItems resolves access items for preview/testing.
	// API: POST /api/v1/access/resolve
	ResolveAccessItems(ctx context.Context, req api.ResolveAccessItemsRequest) (*api.ResolveAccessItemsResponse, error)

	// UploadClipboard stages an image in the container's clipboard directory.
	// API: POST /api/v1/projects/{projectId}/{agentType}/clipboard
	UploadClipboard(ctx context.Context, projectID, agentType string, content []byte, mimeType string) (*api.ClipboardUploadResponse, error)

	// Shutdown requests a graceful server shutdown.
	// API: POST /api/v1/shutdown
	Shutdown(ctx context.Context) error

	// SubscribeEvents returns a channel of real-time SSE events and an
	// unsubscribe function. The channel is closed when the context is
	// cancelled or the connection drops.
	// API: GET /api/v1/events (SSE)
	SubscribeEvents(ctx context.Context) (<-chan eventbus.SSEEvent, func(), error)

	// AttachTerminal returns a bidirectional connection to a worktree's
	// tmux session. In embedded mode this uses docker exec; in HTTP
	// mode this uses WebSocket to /api/v1/projects/{id}/{agentType}/ws/{wid}.
	AttachTerminal(ctx context.Context, projectID, agentType, worktreeID string) (client.TerminalConnection, error)
}

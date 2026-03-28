package tui

import (
	"context"
	"fmt"
	"sync"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/client"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/runtime"
)

// Compile-time check: ServiceAdapter must satisfy Client.
var _ Client = (*ServiceAdapter)(nil)

// containerUser is the non-root user inside Warden containers.
const containerUser = "dev"

// ServiceAdapter wraps a [warden.App] to satisfy the [Client] interface
// for embedded mode (single-process deployment). Most methods delegate
// directly to app.Service. The two exceptions are:
//
//   - [ServiceAdapter.SubscribeEvents]: subscribes to the in-process event
//     broker directly (no SSE/HTTP involved)
//   - [ServiceAdapter.AttachTerminal]: creates a docker exec session attached
//     to the worktree's abduco viewer (no WebSocket involved)
//
// This is the counterpart to [client.Client] (HTTP mode). Both satisfy
// the same [Client] interface, so the TUI works identically in either mode.
//
// Usage:
//
//	app, _ := warden.New(warden.Options{})
//	defer app.Close()
//	adapter := tui.NewServiceAdapter(app)
//	// adapter satisfies tui.Client
type ServiceAdapter struct {
	app *warden.App
}

// NewServiceAdapter creates a [Client] backed by an embedded [warden.App].
func NewServiceAdapter(app *warden.App) *ServiceAdapter {
	return &ServiceAdapter{app: app}
}

// resolveProject looks up the project DB row for a project ID.
// Returns an error if the project is not found.
func (a *ServiceAdapter) resolveProject(projectID string) (*db.ProjectRow, error) {
	row, err := a.app.Service.GetProject(projectID)
	if err != nil {
		return nil, fmt.Errorf("looking up project %q: %w", projectID, err)
	}
	if row == nil {
		return nil, fmt.Errorf("project %q not found", projectID)
	}
	return row, nil
}

// withProject resolves a projectID to a DB project row and calls fn.
// Eliminates the resolve-check-delegate boilerplate across adapter methods.
func withProject[T any](a *ServiceAdapter, projectID string, fn func(project *db.ProjectRow) (T, error)) (T, error) {
	row, err := a.resolveProject(projectID)
	if err != nil {
		var zero T
		return zero, err
	}
	return fn(row)
}

// --- Projects ---

// ListProjects delegates to api.Service.ListProjects.
func (a *ServiceAdapter) ListProjects(ctx context.Context) ([]engine.Project, error) {
	return a.app.Service.ListProjects(ctx)
}

// AddProject delegates to api.Service.AddProject.
func (a *ServiceAdapter) AddProject(_ context.Context, name, hostPath string) (*api.ProjectResult, error) {
	return a.app.Service.AddProject(name, hostPath)
}

// RemoveProject delegates to api.Service.RemoveProject.
func (a *ServiceAdapter) RemoveProject(_ context.Context, projectID string) (*api.ProjectResult, error) {
	return a.app.Service.RemoveProject(projectID)
}

// StopProject resolves the project row and stops the container.
func (a *ServiceAdapter) StopProject(ctx context.Context, projectID string) (*api.ProjectResult, error) {
	return withProject[*api.ProjectResult](a, projectID, func(p *db.ProjectRow) (*api.ProjectResult, error) {
		return a.app.Service.StopProject(ctx, p)
	})
}

// RestartProject resolves the project row and restarts the container.
func (a *ServiceAdapter) RestartProject(ctx context.Context, projectID string) (*api.ProjectResult, error) {
	return withProject[*api.ProjectResult](a, projectID, func(p *db.ProjectRow) (*api.ProjectResult, error) {
		return a.app.Service.RestartProject(ctx, p)
	})
}

// --- Worktrees ---

// ListWorktrees resolves the project row and lists worktrees.
func (a *ServiceAdapter) ListWorktrees(ctx context.Context, projectID string) ([]engine.Worktree, error) {
	return withProject[[]engine.Worktree](a, projectID, func(p *db.ProjectRow) ([]engine.Worktree, error) {
		return a.app.Service.ListWorktrees(ctx, p)
	})
}

// CreateWorktree resolves the project row and creates a worktree.
func (a *ServiceAdapter) CreateWorktree(ctx context.Context, projectID, name string) (*api.WorktreeResult, error) {
	return withProject[*api.WorktreeResult](a, projectID, func(p *db.ProjectRow) (*api.WorktreeResult, error) {
		return a.app.Service.CreateWorktree(ctx, p, name)
	})
}

// ConnectTerminal resolves the project row and connects a terminal.
func (a *ServiceAdapter) ConnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	return withProject[*api.WorktreeResult](a, projectID, func(p *db.ProjectRow) (*api.WorktreeResult, error) {
		return a.app.Service.ConnectTerminal(ctx, p, worktreeID)
	})
}

// DisconnectTerminal resolves the project row and disconnects the terminal.
func (a *ServiceAdapter) DisconnectTerminal(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	return withProject[*api.WorktreeResult](a, projectID, func(p *db.ProjectRow) (*api.WorktreeResult, error) {
		return a.app.Service.DisconnectTerminal(ctx, p, worktreeID)
	})
}

// KillWorktreeProcess resolves the project row and kills the worktree process.
func (a *ServiceAdapter) KillWorktreeProcess(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	return withProject[*api.WorktreeResult](a, projectID, func(p *db.ProjectRow) (*api.WorktreeResult, error) {
		return a.app.Service.KillWorktreeProcess(ctx, p, worktreeID)
	})
}

// RemoveWorktree resolves the project row and removes the worktree.
func (a *ServiceAdapter) RemoveWorktree(ctx context.Context, projectID, worktreeID string) (*api.WorktreeResult, error) {
	return withProject[*api.WorktreeResult](a, projectID, func(p *db.ProjectRow) (*api.WorktreeResult, error) {
		return a.app.Service.RemoveWorktree(ctx, p, worktreeID)
	})
}

// CleanupWorktrees resolves the project row and cleans up orphaned worktrees.
func (a *ServiceAdapter) CleanupWorktrees(ctx context.Context, projectID string) ([]string, error) {
	return withProject[[]string](a, projectID, func(p *db.ProjectRow) ([]string, error) {
		return a.app.Service.CleanupWorktrees(ctx, p)
	})
}

// GetWorktreeDiff resolves the project row and returns the diff.
func (a *ServiceAdapter) GetWorktreeDiff(ctx context.Context, projectID, worktreeID string) (*api.DiffResponse, error) {
	return withProject[*api.DiffResponse](a, projectID, func(p *db.ProjectRow) (*api.DiffResponse, error) {
		return a.app.Service.GetWorktreeDiff(ctx, p, worktreeID)
	})
}

// ResetProjectCosts delegates to service.ResetProjectCosts.
func (a *ServiceAdapter) ResetProjectCosts(_ context.Context, projectID string) error {
	return a.app.Service.ResetProjectCosts(projectID)
}

// PurgeProjectAudit delegates to service.PurgeProjectAudit.
func (a *ServiceAdapter) PurgeProjectAudit(_ context.Context, projectID string) error {
	_, err := a.app.Service.PurgeProjectAudit(projectID)
	return err
}

// --- Containers ---

// CreateContainer delegates to service.CreateContainer.
// The projectID parameter is used by the HTTP client but ignored here —
// the service computes the project ID from req.ProjectPath.
func (a *ServiceAdapter) CreateContainer(_ context.Context, _ string, req engine.CreateContainerRequest) (*api.ContainerResult, error) {
	return a.app.Service.CreateContainer(context.Background(), req)
}

// DeleteContainer resolves the project row and deletes the container.
func (a *ServiceAdapter) DeleteContainer(ctx context.Context, projectID string) (*api.ContainerResult, error) {
	return withProject[*api.ContainerResult](a, projectID, func(p *db.ProjectRow) (*api.ContainerResult, error) {
		return a.app.Service.DeleteContainer(ctx, p)
	})
}

// InspectContainer resolves the project row and returns config.
func (a *ServiceAdapter) InspectContainer(ctx context.Context, projectID string) (*engine.ContainerConfig, error) {
	return withProject[*engine.ContainerConfig](a, projectID, func(p *db.ProjectRow) (*engine.ContainerConfig, error) {
		return a.app.Service.InspectContainer(ctx, p)
	})
}

// UpdateContainer resolves the project row and recreates it.
func (a *ServiceAdapter) UpdateContainer(ctx context.Context, projectID string, req engine.CreateContainerRequest) (*api.ContainerResult, error) {
	return withProject[*api.ContainerResult](a, projectID, func(p *db.ProjectRow) (*api.ContainerResult, error) {
		return a.app.Service.UpdateContainer(ctx, p, req)
	})
}

// ValidateContainer resolves the project row and delegates to service.ValidateContainer.
func (a *ServiceAdapter) ValidateContainer(ctx context.Context, projectID string) (*api.ValidateContainerResult, error) {
	return withProject[*api.ValidateContainerResult](a, projectID, func(p *db.ProjectRow) (*api.ValidateContainerResult, error) {
		return a.app.Service.ValidateContainer(ctx, p)
	})
}

// --- Settings ---

// GetSettings delegates to api.Service.GetSettings.
func (a *ServiceAdapter) GetSettings(_ context.Context) (*api.SettingsResponse, error) {
	resp := a.app.Service.GetSettings()
	return &resp, nil
}

// UpdateSettings delegates to api.Service.UpdateSettings.
func (a *ServiceAdapter) UpdateSettings(ctx context.Context, req api.UpdateSettingsRequest) (*api.UpdateSettingsResult, error) {
	return a.app.Service.UpdateSettings(ctx, req)
}

// --- Host Utilities ---

// GetDefaults delegates to api.Service.GetDefaults.
func (a *ServiceAdapter) GetDefaults(_ context.Context) (*api.DefaultsResponse, error) {
	resp := a.app.Service.GetDefaults()
	return &resp, nil
}

// ListDirectories delegates to api.Service.ListDirectories.
func (a *ServiceAdapter) ListDirectories(_ context.Context, path string, includeFiles bool) ([]api.DirEntry, error) {
	return a.app.Service.ListDirectories(path, includeFiles)
}

// ListRuntimes delegates to api.Service.ListRuntimes.
func (a *ServiceAdapter) ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error) {
	return a.app.Service.ListRuntimes(ctx), nil
}

// --- Audit Log ---

// GetAuditLog delegates to service.GetAuditLog.
func (a *ServiceAdapter) GetAuditLog(_ context.Context, filters api.AuditFilters) ([]db.Entry, error) {
	return a.app.Service.GetAuditLog(filters)
}

// GetAuditSummary delegates to service.GetAuditSummary.
func (a *ServiceAdapter) GetAuditSummary(ctx context.Context, filters api.AuditFilters) (*api.AuditSummary, error) {
	return a.app.Service.GetAuditSummary(ctx, filters)
}

// GetAuditProjects delegates to service.GetAuditProjects.
func (a *ServiceAdapter) GetAuditProjects(_ context.Context) ([]string, error) {
	return a.app.Service.GetAuditProjects()
}

// PostAuditEvent delegates to service.PostAuditEvent.
func (a *ServiceAdapter) PostAuditEvent(_ context.Context, req api.PostAuditEventRequest) error {
	return a.app.Service.PostAuditEvent(req)
}

// DeleteAuditEvents delegates to service.DeleteAuditEvents.
func (a *ServiceAdapter) DeleteAuditEvents(_ context.Context, filters api.AuditFilters) error {
	_, err := a.app.Service.DeleteAuditEvents(filters)
	return err
}

// --- Real-time Events ---

// SubscribeEvents subscribes to the event broker directly (no SSE).
func (a *ServiceAdapter) SubscribeEvents(_ context.Context) (<-chan eventbus.SSEEvent, func(), error) {
	ch, unsub := a.app.Broker.Subscribe()
	return ch, unsub, nil
}

// --- Terminal Attachment ---

// AttachTerminal creates a docker exec session attached to the
// worktree's abduco viewer. This replicates the pattern from
// internal/terminal/proxy.go but returns an io.ReadWriteCloser
// instead of bridging to WebSocket.
func (a *ServiceAdapter) AttachTerminal(ctx context.Context, projectID, worktreeID string) (client.TerminalConnection, error) {
	api := a.app.Engine.APIClient()

	sessionName := fmt.Sprintf("warden-%s", worktreeID)
	execResp, err := api.ContainerExecCreate(ctx, projectID, container.ExecOptions{
		Cmd:          []string{"abduco", "-a", sessionName},
		User:         containerUser,
		Env:          []string{"TERM=xterm-256color"},
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		Tty:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec create: %w", err)
	}

	hijacked, err := api.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &dockerTerminalConn{
		hijacked: &hijacked,
		execID:   execResp.ID,
		api:      api,
		ctx:      ctx,
	}, nil
}

// dockerTerminalConn wraps a hijacked docker exec connection.
type dockerTerminalConn struct {
	hijacked *dtypes.HijackedResponse
	execID   string
	api      execResizer
	ctx      context.Context
	mu       sync.Mutex
	closed   bool
}

// execResizer is the subset of the Docker API needed for resize.
type execResizer interface {
	ContainerExecResize(ctx context.Context, execID string, options container.ResizeOptions) error
}

// Read reads PTY output from the exec session.
func (c *dockerTerminalConn) Read(p []byte) (int, error) {
	return c.hijacked.Reader.Read(p)
}

// Write sends input to the exec session's stdin.
func (c *dockerTerminalConn) Write(p []byte) (int, error) {
	return c.hijacked.Conn.Write(p)
}

// Close terminates the exec connection.
func (c *dockerTerminalConn) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return nil
	}
	c.closed = true
	c.hijacked.Close()
	return nil
}

// Resize changes the PTY dimensions of the exec session.
func (c *dockerTerminalConn) Resize(cols, rows uint) error {
	return c.api.ContainerExecResize(c.ctx, c.execID, container.ResizeOptions{
		Width:  cols,
		Height: rows,
	})
}

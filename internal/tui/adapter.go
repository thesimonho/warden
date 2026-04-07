package tui

import (
	"context"
	"fmt"
	"sync"

	dtypes "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/client"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/runtime"
)

// Compile-time check: ServiceAdapter must satisfy Client.
var _ Client = (*ServiceAdapter)(nil)

// containerUser references the non-root user inside Warden containers.
var containerUser = engine.ContainerUser

// ServiceAdapter wraps a [warden.Warden] to satisfy the [Client] interface
// for embedded mode (single-process deployment). Since Service methods
// accept project IDs and resolve internally, most adapter methods are
// trivial one-liner delegations.
//
// The two exceptions are:
//
//   - [ServiceAdapter.SubscribeEvents]: subscribes to the in-process event
//     broker directly (no SSE/HTTP involved)
//   - [ServiceAdapter.AttachTerminal]: creates a docker exec session attached
//     to the worktree's tmux session (no WebSocket involved)
//
// This is the counterpart to [client.Client] (HTTP mode). Both satisfy
// the same [Client] interface, so the TUI works identically in either mode.
//
// Usage:
//
//	w, _ := warden.New(warden.Options{})
//	defer w.Close()
//	adapter := tui.NewServiceAdapter(w)
//	// adapter satisfies tui.Client
type ServiceAdapter struct {
	w *warden.Warden
}

// NewServiceAdapter creates a [Client] backed by an embedded [warden.Warden].
func NewServiceAdapter(w *warden.Warden) *ServiceAdapter {
	return &ServiceAdapter{w: w}
}

// --- Projects ---

// ListProjects delegates to Service.ListProjects.
func (a *ServiceAdapter) ListProjects(ctx context.Context) ([]api.ProjectResponse, error) {
	return a.w.Service.ListProjects(ctx)
}

// GetProject delegates to Service.GetProjectDetails.
func (a *ServiceAdapter) GetProject(ctx context.Context, projectID, agentType string) (*api.ProjectResponse, error) {
	return a.w.Service.GetProjectDetails(ctx, projectID, agentType)
}

// GetProjectCosts delegates to Service.GetProjectCosts.
func (a *ServiceAdapter) GetProjectCosts(ctx context.Context, projectID, agentType string) (*api.ProjectCostsResponse, error) {
	return a.w.Service.GetProjectCosts(ctx, projectID, agentType)
}

// GetBudgetStatus delegates to Service.GetBudgetStatus.
func (a *ServiceAdapter) GetBudgetStatus(ctx context.Context, projectID, agentType string) (*api.BudgetStatusResponse, error) {
	return a.w.Service.GetBudgetStatus(ctx, projectID, agentType)
}

// BatchProjectOperation delegates to Service.BatchProjectOperation.
func (a *ServiceAdapter) BatchProjectOperation(ctx context.Context, req api.BatchProjectRequest) (*api.BatchProjectResponse, error) {
	return a.w.Service.BatchProjectOperation(ctx, req), nil
}

// AddProject delegates to Service.AddProjectWithContainer.
func (a *ServiceAdapter) AddProject(ctx context.Context, req api.AddProjectRequest) (*api.AddProjectResponse, error) {
	return a.w.Service.AddProjectWithContainer(ctx, req)
}

// RemoveProject delegates to Service.RemoveProject.
func (a *ServiceAdapter) RemoveProject(_ context.Context, projectID, agentType string) (*api.ProjectResult, error) {
	return a.w.Service.RemoveProject(projectID, agentType)
}

// StopProject delegates to Service.StopProject.
func (a *ServiceAdapter) StopProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error) {
	return a.w.Service.StopProject(ctx, projectID, agentType)
}

// RestartProject delegates to Service.RestartProject.
func (a *ServiceAdapter) RestartProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error) {
	return a.w.Service.RestartProject(ctx, projectID, agentType)
}

// --- Worktrees ---

// ListWorktrees delegates to Service.ListWorktrees.
func (a *ServiceAdapter) ListWorktrees(ctx context.Context, projectID, agentType string) ([]engine.Worktree, error) {
	return a.w.Service.ListWorktrees(ctx, projectID, agentType)
}

// GetWorktree delegates to Service.GetWorktree.
func (a *ServiceAdapter) GetWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*engine.Worktree, error) {
	return a.w.Service.GetWorktree(ctx, projectID, agentType, worktreeID)
}

// SendWorktreeInput delegates to Service.SendWorktreeInput.
func (a *ServiceAdapter) SendWorktreeInput(ctx context.Context, projectID, agentType, worktreeID string, req api.WorktreeInputRequest) error {
	return a.w.Service.SendWorktreeInput(ctx, projectID, agentType, worktreeID, req)
}

// CreateWorktree delegates to Service.CreateWorktree.
func (a *ServiceAdapter) CreateWorktree(ctx context.Context, projectID, agentType string, req api.CreateWorktreeRequest) (*api.WorktreeResult, error) {
	return a.w.Service.CreateWorktree(ctx, projectID, agentType, req.Name)
}

// ConnectTerminal delegates to Service.ConnectTerminal.
func (a *ServiceAdapter) ConnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	return a.w.Service.ConnectTerminal(ctx, projectID, agentType, worktreeID)
}

// DisconnectTerminal delegates to Service.DisconnectTerminal.
func (a *ServiceAdapter) DisconnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	return a.w.Service.DisconnectTerminal(ctx, projectID, agentType, worktreeID)
}

// KillWorktreeProcess delegates to Service.KillWorktreeProcess.
func (a *ServiceAdapter) KillWorktreeProcess(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	return a.w.Service.KillWorktreeProcess(ctx, projectID, agentType, worktreeID)
}

// ResetWorktree delegates to Service.ResetWorktree.
func (a *ServiceAdapter) ResetWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	return a.w.Service.ResetWorktree(ctx, projectID, agentType, worktreeID)
}

// RemoveWorktree delegates to Service.RemoveWorktree.
func (a *ServiceAdapter) RemoveWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	return a.w.Service.RemoveWorktree(ctx, projectID, agentType, worktreeID)
}

// CleanupWorktrees delegates to Service.CleanupWorktrees.
func (a *ServiceAdapter) CleanupWorktrees(ctx context.Context, projectID, agentType string) ([]string, error) {
	return a.w.Service.CleanupWorktrees(ctx, projectID, agentType)
}

// GetWorktreeDiff delegates to Service.GetWorktreeDiff.
func (a *ServiceAdapter) GetWorktreeDiff(ctx context.Context, projectID, agentType, worktreeID string) (*api.DiffResponse, error) {
	return a.w.Service.GetWorktreeDiff(ctx, projectID, agentType, worktreeID)
}

// ResetProjectCosts delegates to Service.ResetProjectCosts.
func (a *ServiceAdapter) ResetProjectCosts(_ context.Context, projectID, agentType string) error {
	return a.w.Service.ResetProjectCosts(projectID, agentType)
}

// PurgeProjectAudit delegates to Service.PurgeProjectAudit.
func (a *ServiceAdapter) PurgeProjectAudit(_ context.Context, projectID, agentType string) error {
	_, err := a.w.Service.PurgeProjectAudit(projectID, agentType)
	return err
}

// --- Containers ---

// CreateContainer delegates to Service.CreateContainer.
// The projectID parameter is used by the HTTP client but ignored here —
// the service computes the project ID from req.ProjectPath.
func (a *ServiceAdapter) CreateContainer(_ context.Context, _, _ string, req api.CreateContainerRequest) (*api.ContainerResult, error) {
	return a.w.Service.CreateContainer(context.Background(), req)
}

// DeleteContainer delegates to Service.DeleteContainer.
func (a *ServiceAdapter) DeleteContainer(ctx context.Context, projectID, agentType string) (*api.ContainerResult, error) {
	return a.w.Service.DeleteContainer(ctx, projectID, agentType)
}

// InspectContainer delegates to Service.InspectContainer.
func (a *ServiceAdapter) InspectContainer(ctx context.Context, projectID, agentType string) (*api.ContainerConfig, error) {
	return a.w.Service.InspectContainer(ctx, projectID, agentType)
}

// UpdateContainer delegates to Service.UpdateContainer.
func (a *ServiceAdapter) UpdateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*api.ContainerResult, error) {
	return a.w.Service.UpdateContainer(ctx, projectID, agentType, req)
}

// ValidateContainer delegates to Service.ValidateContainer.
func (a *ServiceAdapter) ValidateContainer(ctx context.Context, projectID, agentType string) (*api.ValidateContainerResult, error) {
	return a.w.Service.ValidateContainer(ctx, projectID, agentType)
}

// --- Settings ---

// GetSettings delegates to Service.GetSettings.
func (a *ServiceAdapter) GetSettings(_ context.Context) (*api.SettingsResponse, error) {
	resp := a.w.Service.GetSettings()
	return &resp, nil
}

// UpdateSettings delegates to Service.UpdateSettings.
func (a *ServiceAdapter) UpdateSettings(ctx context.Context, req api.UpdateSettingsRequest) (*api.UpdateSettingsResult, error) {
	return a.w.Service.UpdateSettings(ctx, req)
}

// --- Host Utilities ---

// GetDefaults delegates to Service.GetDefaults.
func (a *ServiceAdapter) GetDefaults(_ context.Context, projectPath string) (*api.DefaultsResponse, error) {
	resp := a.w.Service.GetDefaults(projectPath)
	return &resp, nil
}

// ReadProjectTemplate delegates to Service.ReadProjectTemplate.
func (a *ServiceAdapter) ReadProjectTemplate(_ context.Context, filePath string) (*api.ProjectTemplate, error) {
	return a.w.Service.ReadProjectTemplate(filePath)
}

// ValidateProjectTemplate delegates to Service.ValidateProjectTemplate.
func (a *ServiceAdapter) ValidateProjectTemplate(_ context.Context, data []byte) (*api.ProjectTemplate, error) {
	return a.w.Service.ValidateProjectTemplate(data)
}

// ListDirectories delegates to Service.ListDirectories.
func (a *ServiceAdapter) ListDirectories(_ context.Context, path string, includeFiles bool) ([]api.DirEntry, error) {
	return a.w.Service.ListDirectories(path, includeFiles)
}

// ListRuntimes delegates to Service.ListRuntimes.
func (a *ServiceAdapter) ListRuntimes(ctx context.Context) ([]runtime.RuntimeInfo, error) {
	return a.w.Service.ListRuntimes(ctx), nil
}

// --- Audit Log ---

// GetAuditLog delegates to Service.GetAuditLog.
func (a *ServiceAdapter) GetAuditLog(_ context.Context, filters api.AuditFilters) ([]api.AuditEntry, error) {
	return a.w.Service.GetAuditLog(filters)
}

// GetAuditSummary delegates to Service.GetAuditSummary.
func (a *ServiceAdapter) GetAuditSummary(ctx context.Context, filters api.AuditFilters) (*api.AuditSummary, error) {
	return a.w.Service.GetAuditSummary(ctx, filters)
}

// GetAuditProjects delegates to Service.GetAuditProjects.
func (a *ServiceAdapter) GetAuditProjects(_ context.Context) ([]string, error) {
	return a.w.Service.GetAuditProjects()
}

// PostAuditEvent delegates to Service.PostAuditEvent.
func (a *ServiceAdapter) PostAuditEvent(_ context.Context, req api.PostAuditEventRequest) error {
	return a.w.Service.PostAuditEvent(req)
}

// DeleteAuditEvents delegates to Service.DeleteAuditEvents.
func (a *ServiceAdapter) DeleteAuditEvents(_ context.Context, filters api.AuditFilters) error {
	_, err := a.w.Service.DeleteAuditEvents(filters)
	return err
}

// --- Access Items ---

// ListAccessItems delegates to Service.ListAccessItems.
func (a *ServiceAdapter) ListAccessItems(_ context.Context) (*api.AccessItemListResponse, error) {
	items, err := a.w.Service.ListAccessItems()
	if err != nil {
		return nil, err
	}
	return &api.AccessItemListResponse{Items: items}, nil
}

// GetAccessItem delegates to Service.GetAccessItem.
func (a *ServiceAdapter) GetAccessItem(_ context.Context, id string) (*api.AccessItemResponse, error) {
	return a.w.Service.GetAccessItem(id)
}

// CreateAccessItem delegates to Service.CreateAccessItem.
func (a *ServiceAdapter) CreateAccessItem(_ context.Context, req api.CreateAccessItemRequest) (*access.Item, error) {
	return a.w.Service.CreateAccessItem(req)
}

// UpdateAccessItem delegates to Service.UpdateAccessItem.
func (a *ServiceAdapter) UpdateAccessItem(_ context.Context, id string, req api.UpdateAccessItemRequest) (*access.Item, error) {
	return a.w.Service.UpdateAccessItem(id, req)
}

// DeleteAccessItem delegates to Service.DeleteAccessItem.
func (a *ServiceAdapter) DeleteAccessItem(_ context.Context, id string) error {
	return a.w.Service.DeleteAccessItem(id)
}

// ResetAccessItem delegates to Service.ResetAccessItem.
func (a *ServiceAdapter) ResetAccessItem(_ context.Context, id string) (*access.Item, error) {
	return a.w.Service.ResetAccessItem(id)
}

// ResolveAccessItems delegates to Service.ResolveAccessItems.
func (a *ServiceAdapter) ResolveAccessItems(_ context.Context, req api.ResolveAccessItemsRequest) (*api.ResolveAccessItemsResponse, error) {
	return a.w.Service.ResolveAccessItems(req.Items)
}

// --- Clipboard ---

// UploadClipboard delegates to Service.UploadClipboard.
func (a *ServiceAdapter) UploadClipboard(ctx context.Context, projectID, agentType string, content []byte, mimeType string) (*api.ClipboardUploadResponse, error) {
	return a.w.Service.UploadClipboard(ctx, projectID, agentType, content, mimeType)
}

// --- Server Lifecycle ---

// Shutdown is a no-op in embedded mode. The TUI process exits via
// tea.Quit, which unwinds main() and calls w.Close() via defer.
func (a *ServiceAdapter) Shutdown(_ context.Context) error {
	return nil
}

// BaseURL returns an empty string in embedded mode since there is no
// HTTP server running. Port forwarding proxy URLs are only available
// when the TUI connects to a remote Warden server via client.Client.
func (a *ServiceAdapter) BaseURL() string {
	return ""
}

// --- Real-time Events ---

// SubscribeEvents subscribes to the event broker directly (no SSE).
// Filter options are ignored in embedded mode — the TUI receives all events.
func (a *ServiceAdapter) SubscribeEvents(_ context.Context, _ ...client.SubscribeEventsOptions) (<-chan eventbus.SSEEvent, func(), error) {
	ch, unsub := a.w.Broker.Subscribe()
	return ch, unsub, nil
}

// --- Terminal Attachment ---

// AttachTerminal creates a docker exec session attached to the
// worktree's tmux session. This replicates the pattern from
// internal/terminal/proxy.go but returns an io.ReadWriteCloser
// instead of bridging to WebSocket.
func (a *ServiceAdapter) AttachTerminal(ctx context.Context, projectID, agentType, worktreeID string) (client.TerminalConnection, error) {
	dockerAPI := a.w.Engine.APIClient()

	sessionName := engine.TmuxSessionName(worktreeID)
	execResp, err := dockerAPI.ContainerExecCreate(ctx, projectID, container.ExecOptions{
		Cmd:          []string{"tmux", "-u", "attach-session", "-t", sessionName},
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

	hijacked, err := dockerAPI.ContainerExecAttach(ctx, execResp.ID, container.ExecStartOptions{
		Tty: true,
	})
	if err != nil {
		return nil, fmt.Errorf("exec attach: %w", err)
	}

	return &dockerTerminalConn{
		hijacked: &hijacked,
		execID:   execResp.ID,
		api:      dockerAPI,
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

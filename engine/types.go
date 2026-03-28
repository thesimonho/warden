// Package engine wraps the Docker Engine API for discovering and managing
// Claude Code project containers.
package engine

import (
	"context"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
)

// ClaudeStatus represents whether Claude Code is actively running inside a container.
type ClaudeStatus string

const (
	// ClaudeStatusIdle means no Claude process is running.
	ClaudeStatusIdle ClaudeStatus = "idle"
	// ClaudeStatusWorking means a Claude process is currently active.
	ClaudeStatusWorking ClaudeStatus = "working"
	// ClaudeStatusUnknown means the status could not be determined.
	ClaudeStatusUnknown ClaudeStatus = "unknown"
)

// NotificationType represents the kind of attention Claude Code needs from the user.
type NotificationType string

const (
	// NotificationPermissionPrompt means Claude needs tool approval.
	NotificationPermissionPrompt NotificationType = "permission_prompt"
	// NotificationIdlePrompt means Claude is done and waiting for the next prompt.
	NotificationIdlePrompt NotificationType = "idle_prompt"
	// NotificationAuthSuccess means authentication just completed.
	NotificationAuthSuccess NotificationType = "auth_success"
	// NotificationElicitationDialog means Claude is asking the user a question.
	NotificationElicitationDialog NotificationType = "elicitation_dialog"
)

// notificationPriority returns a numeric priority for notification types.
// Higher values indicate more urgent attention (permission_prompt > elicitation > idle).
func notificationPriority(nt NotificationType) int {
	switch nt {
	case NotificationPermissionPrompt:
		return 3
	case NotificationElicitationDialog:
		return 2
	case NotificationIdlePrompt:
		return 1
	default:
		return 0
	}
}

// WorktreeState represents the terminal connection state of a worktree.
type WorktreeState string

const (
	// WorktreeStateConnected means a terminal is running with Claude active.
	WorktreeStateConnected WorktreeState = "connected"
	// WorktreeStateShell means Claude exited but the bash shell is still alive via ttyd.
	WorktreeStateShell WorktreeState = "shell"
	// WorktreeStateBackground means the abduco session is alive but ttyd is not
	// serving (e.g. browser closed). Claude Code may still be working.
	WorktreeStateBackground WorktreeState = "background"
	// WorktreeStateDisconnected means no terminal process is running.
	WorktreeStateDisconnected WorktreeState = "disconnected"
)

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

// Project represents a project tracked by Warden, optionally backed by a Docker container.
// ProjectID is the stable identity (deterministic hash of host path).
// ID is the Docker container ID (empty when HasContainer is false).
type Project struct {
	// ProjectID is the deterministic project identifier (sha256 of host path, 12 hex chars).
	ProjectID string `json:"projectId"`
	// ID is the Docker container ID (empty when no container exists).
	ID string `json:"id"`
	// Name is the user-chosen display label / Docker container name.
	Name string `json:"name"`
	// HostPath is the absolute host directory mounted into the container.
	HostPath string `json:"hostPath,omitempty"`
	// HasContainer is true when a Docker container is associated with this project.
	HasContainer bool         `json:"hasContainer"`
	Type         string       `json:"type"`
	Image        string       `json:"image"`
	OS           string       `json:"os"`
	CreatedAt    int64        `json:"createdAt"`
	SSHPort      string       `json:"sshPort"`
	State        string       `json:"state"`
	Status       string       `json:"status"`
	ClaudeStatus ClaudeStatus `json:"claudeStatus"`
	// NeedsInput is true when any worktree requires user attention.
	NeedsInput bool `json:"needsInput,omitempty"`
	// NotificationType indicates why Claude needs attention (e.g. permission_prompt, idle_prompt).
	NotificationType NotificationType `json:"notificationType,omitempty"`
	// ActiveWorktreeCount is the number of worktrees with connected terminals.
	ActiveWorktreeCount int `json:"activeWorktreeCount"`
	// TotalCost is the aggregate cost across all worktrees in USD (from agent status provider).
	TotalCost float64 `json:"totalCost"`
	// IsEstimatedCost is true when the cost is an estimate (e.g. subscription users).
	// When false, the cost reflects actual API spend.
	IsEstimatedCost bool `json:"isEstimatedCost,omitempty"`
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64 `json:"costBudget"`
	// IsGitRepo indicates whether the container's /project is a git repository.
	IsGitRepo bool `json:"isGitRepo"`
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
}

// Worktree represents a git worktree (or implicit project root) with its terminal state.
type Worktree struct {
	// ID is the worktree identifier — directory name for git worktrees, "main" for project root.
	ID string `json:"id"`
	// ProjectID is the container ID this worktree belongs to.
	ProjectID string `json:"projectId"`
	// Path is the filesystem path inside the container.
	Path string `json:"path"`
	// Branch is the git branch checked out in this worktree.
	Branch string `json:"branch,omitempty"`
	// State is the terminal connection state (connected, shell, disconnected).
	State WorktreeState `json:"state"`
	// ExitCode is Claude's exit code when in shell state.
	ExitCode int `json:"exitCode,omitempty"`
	// NeedsInput is true when Claude is blocked waiting for user attention.
	NeedsInput bool `json:"needsInput,omitempty"`
	// NotificationType indicates why Claude needs attention.
	NotificationType NotificationType `json:"notificationType,omitempty"`
}

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
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	ProjectPath string            `json:"projectPath"`
	EnvVars     map[string]string `json:"envVars,omitempty"`
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
}

// ContainerConfig holds the editable configuration of an existing container.
// Returned by InspectContainer for populating the edit form.
type ContainerConfig struct {
	Name            string            `json:"name"`
	Image           string            `json:"image"`
	ProjectPath     string            `json:"projectPath"`
	EnvVars         map[string]string `json:"envVars,omitempty"`
	Mounts          []Mount           `json:"mounts,omitempty"`
	SkipPermissions bool              `json:"skipPermissions"`
	// NetworkMode controls the container's network isolation level.
	NetworkMode NetworkMode `json:"networkMode"`
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// CostBudget is the per-project cost limit in USD (0 = use global default).
	CostBudget float64 `json:"costBudget"`
}

// Client defines the interface for interacting with Docker containers.
// All methods accept a context for cancellation and timeout control.
type Client interface {
	// ListProjects returns projects for the given container names, enriched with
	// live Docker state. Names not found in Docker are returned with HasContainer: false.
	ListProjects(ctx context.Context, names []string) ([]Project, error)

	// StopProject gracefully stops the container with the given ID.
	StopProject(ctx context.Context, id string) error

	// RestartProject restarts the container with the given ID.
	// originalMounts are the pre-symlink-resolution mount specs from the DB,
	// used to detect stale bind mounts before restarting.
	RestartProject(ctx context.Context, id string, originalMounts []Mount) error

	// CreateContainer creates and starts a new project container.
	CreateContainer(ctx context.Context, req CreateContainerRequest) (string, error)

	// DeleteContainer stops and removes a container.
	DeleteContainer(ctx context.Context, id string) error

	// CleanupEventDir removes the bind-mounted event directory for a container.
	CleanupEventDir(containerName string)

	// InspectContainer returns the editable configuration of a container.
	InspectContainer(ctx context.Context, id string) (*ContainerConfig, error)

	// RecreateContainer replaces a stopped container with a new one using updated config.
	// Returns the new container ID.
	RecreateContainer(ctx context.Context, id string, req CreateContainerRequest) (string, error)

	// ListWorktrees returns all worktrees for the given container with their terminal state.
	// When skipEnrich is true, the expensive batch docker exec for terminal state is skipped
	// (the caller is expected to overlay state from the event bus store instead).
	ListWorktrees(ctx context.Context, containerID string, skipEnrich bool) ([]Worktree, error)

	// CreateWorktree creates a new git worktree inside the container and connects a terminal.
	// When skipPermissions is true, Claude Code runs with --dangerously-skip-permissions.
	// Returns the worktree ID on success.
	CreateWorktree(ctx context.Context, containerID, name string, skipPermissions bool) (string, error)

	// ConnectTerminal starts a terminal for a worktree inside the container.
	// When skipPermissions is true, Claude Code runs with --dangerously-skip-permissions.
	// Returns the worktree ID on success.
	ConnectTerminal(ctx context.Context, containerID, worktreeID string, skipPermissions bool) (string, error)

	// DisconnectTerminal kills the ttyd viewer for a worktree, freeing the port.
	// The abduco session (and Claude/bash) continues running in the background.
	DisconnectTerminal(ctx context.Context, containerID, worktreeID string) error

	// KillWorktreeProcess kills both ttyd and abduco for a worktree, destroying
	// the process entirely. The git worktree directory on disk is preserved.
	KillWorktreeProcess(ctx context.Context, containerID, worktreeID string) error

	// RemoveWorktree fully removes a worktree: kills any running processes,
	// runs `git worktree remove`, and cleans up tracking state. Cannot remove
	// the "main" worktree.
	RemoveWorktree(ctx context.Context, containerID, worktreeID string) error

	// CleanupOrphanedWorktrees removes worktree directories from .claude/worktrees/
	// that are not tracked by git. Returns the list of removed worktree IDs.
	CleanupOrphanedWorktrees(ctx context.Context, containerID string) ([]string, error)

	// ValidateInfrastructure checks whether a container has the required Warden
	// terminal infrastructure installed (ttyd, abduco, create-terminal.sh).
	// Returns true if all binaries are present, along with the list of missing items.
	ValidateInfrastructure(ctx context.Context, containerID string) (bool, []string, error)

	// GetWorktreeDiff returns uncommitted changes (tracked + untracked) for a
	// worktree inside the container as a unified diff with per-file statistics.
	GetWorktreeDiff(ctx context.Context, containerID, worktreeID string) (*api.DiffResponse, error)

	// ReadAgentStatus reads the agent config file from a running container
	// and returns per-project status data keyed by working directory path.
	// Used as a fallback cost source when the event bus has no data.
	ReadAgentStatus(ctx context.Context, containerID string) (map[string]*agent.Status, error)

	// IsEstimatedCost returns true when a container's cost is estimated
	// (subscription user) rather than actual API spend (API key user).
	IsEstimatedCost(ctx context.Context, containerID string) bool

	// ReadAgentCostAndBillingType reads the agent config once and returns
	// both cost (filtered by workspace prefix) and billing type.
	ReadAgentCostAndBillingType(ctx context.Context, containerID, workspacePrefix string) (*AgentCostResult, error)
}

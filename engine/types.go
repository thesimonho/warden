// Package engine wraps the Docker Engine API for discovering and managing
// Claude Code project containers.
package engine

import (
	"context"
	"io"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
)

// AgentStatus represents whether the agent CLI is actively running inside a container.
type AgentStatus string

const (
	// AgentStatusIdle means no agent process is running.
	AgentStatusIdle AgentStatus = "idle"
	// AgentStatusWorking means an agent process is currently active.
	AgentStatusWorking AgentStatus = "working"
	// AgentStatusUnknown means the status could not be determined.
	AgentStatusUnknown AgentStatus = "unknown"
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

// NotificationPriority returns a numeric priority for notification types.
// Higher values indicate more urgent attention (permission_prompt > elicitation > idle).
func NotificationPriority(nt NotificationType) int {
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
	// WorktreeStateShell means the agent exited but the bash shell is still alive.
	WorktreeStateShell WorktreeState = "shell"
	// WorktreeStateBackground means the tmux session is alive but no viewer is
	// connected (e.g. browser closed). Claude Code may still be working.
	WorktreeStateBackground WorktreeState = "background"
	// WorktreeStateStopped means no terminal process is running.
	WorktreeStateStopped WorktreeState = "stopped"
)

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
	HasContainer bool        `json:"hasContainer"`
	Type         string      `json:"type"`
	Image        string      `json:"image"`
	OS           string      `json:"os"`
	CreatedAt    int64       `json:"createdAt"`
	SSHPort      string      `json:"sshPort"`
	State        string      `json:"state"`
	Status       string      `json:"status"`
	AgentStatus  AgentStatus `json:"agentStatus"`
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
	// AgentType identifies the CLI agent running in this project (e.g. "claude-code", "codex").
	AgentType constants.AgentType `json:"agentType"`
	// SkipPermissions indicates whether terminals should skip permission prompts.
	SkipPermissions bool `json:"skipPermissions"`
	// MountedDir is the host directory mounted into the container.
	MountedDir string `json:"mountedDir,omitempty"`
	// WorkspaceDir is the container-side workspace directory (mount destination).
	WorkspaceDir string `json:"workspaceDir,omitempty"`
	// NetworkMode controls the container's network isolation level.
	NetworkMode api.NetworkMode `json:"networkMode"`
	// AllowedDomains lists domains accessible when NetworkMode is "restricted".
	AllowedDomains []string `json:"allowedDomains,omitempty"`
	// AgentVersion is the pinned CLI version installed in this container.
	AgentVersion string `json:"agentVersion,omitempty"`
	// ForwardedPorts lists container ports exposed via the reverse proxy.
	ForwardedPorts []int `json:"forwardedPorts,omitempty"`
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
	// State is the terminal connection state (connected, shell, background, stopped).
	State WorktreeState `json:"state"`
	// ExitCode is the agent's exit code when in shell state.
	// Nil means the agent is still running (or no exit code captured).
	ExitCode *int `json:"exitCode,omitempty"`
	// NeedsInput is true when Claude is blocked waiting for user attention.
	NeedsInput bool `json:"needsInput,omitempty"`
	// NotificationType indicates why Claude needs attention.
	NotificationType NotificationType `json:"notificationType,omitempty"`
}

// ContainerHealth describes a container's startup health state.
// Used by the liveness checker to diagnose crash-looping containers.
type ContainerHealth struct {
	// Restarting is true when the container is in a Docker restart loop.
	Restarting bool
	// RestartCount is the number of times Docker has restarted the container.
	RestartCount int
	// ExitCode is the last exit code from the container's entrypoint.
	ExitCode int
	// OOMKilled is true if the container was killed due to memory limits.
	OOMKilled bool
	// LogTail contains the last lines of container logs (only populated when unhealthy).
	LogTail string
}

// Client defines the interface for interacting with Docker containers.
// All methods accept a context for cancellation and timeout control.
type Client interface {
	// ListProjects returns projects for the given container names, enriched with
	// live Docker state. Names not found in Docker are returned with HasContainer: false.
	ListProjects(ctx context.Context, names []string) ([]Project, error)

	// StopProject gracefully stops the container with the given ID.
	StopProject(ctx context.Context, id string) error

	// RestartProject restarts the container with the given ID and re-applies
	// network isolation if needed. originalMounts are the pre-symlink-resolution
	// mount specs from the DB, used to detect stale bind mounts before restarting.
	// networkMode and allowedDomains are read from the DB so that network
	// isolation can be re-applied after the container restarts.
	RestartProject(ctx context.Context, id string, originalMounts []api.Mount, networkMode string, allowedDomains []string) error

	// CreateContainer creates and starts a new project container.
	CreateContainer(ctx context.Context, req api.CreateContainerRequest) (string, error)

	// DeleteContainer stops and removes a container.
	DeleteContainer(ctx context.Context, id string) error

	// CleanupEventDir removes the bind-mounted event directory for a container.
	CleanupEventDir(containerName string)

	// InspectContainer returns the editable configuration of a container.
	InspectContainer(ctx context.Context, id string) (*api.ContainerConfig, error)

	// ContainerIP returns the bridge network IP address of a running container.
	ContainerIP(ctx context.Context, containerID string) (string, error)

	// RenameContainer changes the name of an existing container without recreation.
	RenameContainer(ctx context.Context, id string, newName string) error

	// ReloadAllowedDomains re-runs the network isolation script inside a
	// running container to update the allowed domain list without recreation.
	// Uses privileged docker exec since the container lacks NET_ADMIN.
	ReloadAllowedDomains(ctx context.Context, containerID string, domains []string) error

	// ApplyNetworkIsolation runs the network isolation script via privileged
	// docker exec. Used after container start/restart to set up iptables
	// without granting NET_ADMIN to the container.
	ApplyNetworkIsolation(ctx context.Context, containerID, mode string, domains []string) error

	// RecreateContainer replaces a stopped container with a new one using updated config.
	// Returns the new container ID.
	RecreateContainer(ctx context.Context, id string, req api.CreateContainerRequest) (string, error)

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

	// DisconnectTerminal pushes a disconnect event and cleans up tracking state.
	// The tmux session (and Claude/bash) continues running in the background.
	DisconnectTerminal(ctx context.Context, containerID, worktreeID string) error

	// KillWorktreeProcess kills the tmux session for a worktree, destroying
	// the process entirely. The git worktree directory on disk is preserved.
	KillWorktreeProcess(ctx context.Context, containerID, worktreeID string) error

	// SendWorktreeInput sends text to a worktree's tmux pane. Uses literal mode
	// to prevent key-name interpretation. If pressEnter is true, sends Enter after the text.
	SendWorktreeInput(ctx context.Context, containerID, worktreeID, text string, pressEnter bool) error

	// ResetWorktree clears all history for a worktree without removing it.
	// Kills the process, clears JSONL session files, and removes terminal
	// tracking state.
	ResetWorktree(ctx context.Context, containerID, worktreeID string) error

	// RemoveWorktree fully removes a worktree: kills any running processes,
	// runs `git worktree remove`, and cleans up tracking state. Cannot remove
	// the "main" worktree.
	RemoveWorktree(ctx context.Context, containerID, worktreeID string) error

	// CleanupOrphanedWorktrees removes worktree directories from .claude/worktrees/
	// that are not tracked by git. Returns the list of removed worktree IDs.
	CleanupOrphanedWorktrees(ctx context.Context, containerID string) ([]string, error)

	// ValidateInfrastructure checks whether a container has the required Warden
	// terminal infrastructure installed (tmux, gosu, create-terminal.sh, etc).
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

	// ContainerStartupHealth inspects a container's state to determine if it
	// is crash-looping. When the container is restarting, reads the last lines
	// of container logs to capture the error. Used by the liveness checker to
	// enrich stale-heartbeat audit events with diagnostic details.
	ContainerStartupHealth(ctx context.Context, containerName string) (*ContainerHealth, error)

	// CopyFileToContainer writes a single file into a running container.
	// Uses exec+stdin (sh -c 'cat > file') instead of the Docker tar archive API.
	// Used by the clipboard upload feature to stage images for the xclip shim.
	CopyFileToContainer(ctx context.Context, containerID, destDir, filename string, content io.Reader, size int64) error
}

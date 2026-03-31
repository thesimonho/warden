package engine

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"

	"github.com/thesimonho/warden/agent"
	cruntime "github.com/thesimonho/warden/runtime"
)

// stopTimeout is the grace period before force-killing a container.
// Gives Claude time to finish writing files and save state.
const stopTimeout = 30 * time.Second

// ContainerWorkspaceDir computes the container-side workspace path for a project.
// New containers mount at /home/warden/<name> to give each project a unique path
// in the agent's config file (which keys cost data by workspace path).
func ContainerWorkspaceDir(projectName string) string {
	return ContainerHomeDir + "/" + projectName
}

// EngineClient wraps the Docker/Podman Engine SDK client for container operations.
type EngineClient struct {
	api                client.APIClient
	agentRegistry      *agent.Registry
	runtimeName        string   // "docker" or "podman"
	eventBaseDir       string   // host-side base directory for event files
	seccompProfilePath string   // host-side path to the seccomp JSON profile file (Podman)
	seccompProfileJSON string   // inline seccomp profile JSON (Docker)
	gitRepoCache       sync.Map // containerID -> bool, cached per container lifetime
	workspaceDirCache  sync.Map // containerID -> string, cached workspace dir
	agentTypeCache     sync.Map // containerID -> string, cached agent type (immutable per container)
}

// NewClient creates an EngineClient using the given socket path.
// The runtimeName ("docker" or "podman") determines runtime-specific
// container configuration (e.g. --userns=keep-id for Podman).
// The registry maps agent type names to StatusProvider implementations.
// When socketPath is empty, falls back to client.FromEnv (default Docker behavior).
//
// The socketPath can be an absolute Unix path (/var/run/docker.sock),
// a Windows named pipe (//./pipe/docker_engine), or a URI with scheme
// (unix://, npipe://, tcp://).
func NewClient(socketPath string, runtimeName string, registry *agent.Registry) (*EngineClient, error) {
	opts := []client.Opt{client.WithAPIVersionNegotiation()}
	if socketPath != "" {
		opts = append(opts, client.WithHost(cruntime.SocketHost(socketPath)))
	} else {
		opts = append(opts, client.FromEnv)
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("creating docker client: %w", err)
	}
	return &EngineClient{
		api:           cli,
		agentRegistry: registry,
		runtimeName:   runtimeName,
	}, nil
}

// APIClient returns the underlying Docker/Podman API client.
// Used by the terminal proxy to create exec sessions with TTY mode.
func (ec *EngineClient) APIClient() client.APIClient {
	return ec.api
}

// SetEventBaseDir configures the host-side base directory for event files.
// Each container gets a subdirectory at <baseDir>/<containerName>/events/
// that is bind-mounted into the container at /var/warden/events/.
func (ec *EngineClient) SetEventBaseDir(dir string) {
	ec.eventBaseDir = dir
}

// CleanupEventDir removes the event directory for a container.
// Called after a container is deleted to prevent orphaned directories.
func (ec *EngineClient) CleanupEventDir(containerName string) {
	if ec.eventBaseDir == "" || containerName == "" {
		return
	}
	dir := filepath.Join(ec.eventBaseDir, containerName)
	_ = os.RemoveAll(dir)
}

// SetSeccompProfile configures the seccomp profile for new containers.
// Docker's API requires inline JSON in SecurityOpt, while Podman requires
// a file path (inline JSON triggers "file name too long" errors).
func (ec *EngineClient) SetSeccompProfile(filePath string, profileJSON string) {
	ec.seccompProfilePath = filePath
	ec.seccompProfileJSON = profileJSON
}

// ListProjects fetches Docker state for the given container names.
// Names not found in Docker are returned with HasContainer: false.
// The returned order matches the input name order.
func (ec *EngineClient) ListProjects(ctx context.Context, names []string) ([]Project, error) {
	if len(names) == 0 {
		return []Project{}, nil
	}

	// Pre-populate all slots as no-container so missing containers are represented.
	nameIndex := make(map[string]int, len(names))
	projects := make([]Project, len(names))
	for i, name := range names {
		nameIndex[name] = i
		projects[i] = Project{Name: name, HasContainer: false, AgentStatus: AgentStatusUnknown}
	}

	// Docker's name filter does prefix matching, so we filter exactly in code.
	filterArgs := filters.NewArgs()
	for _, name := range names {
		filterArgs.Add("name", name)
	}

	containers, err := ec.api.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filterArgs,
	})
	if err != nil {
		return nil, fmt.Errorf("listing containers: %w", err)
	}

	for _, c := range containers {
		name := containerName(c.Names)
		idx, ok := nameIndex[name]
		if !ok {
			continue // prefix match hit an unrelated container
		}
		projects[idx] = containerToProject(c)
	}

	ec.enrichProjectStatus(ctx, projects)

	slog.Info("listed projects", "count", len(projects))
	return projects, nil
}

// enrichProjectStatus fetches worktree data for each running container to derive
// Claude status, worktree counts, and git repo status. Runs in parallel.
// Cost is overlaid separately from the event store at the routes layer.
func (ec *EngineClient) enrichProjectStatus(ctx context.Context, projects []Project) {
	var wg sync.WaitGroup

	for i := range projects {
		if !projects[i].HasContainer || projects[i].State != "running" {
			projects[i].AgentStatus = AgentStatusUnknown
			continue
		}

		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Check if the project is a git repo
			projects[idx].IsGitRepo = ec.checkIsGitRepo(ctx, projects[idx].ID)

			// Derive status from worktrees (pass isGitRepo to avoid duplicate exec)
			worktrees, err := ec.listWorktreesWithHint(ctx, projects[idx].ID, projects[idx].IsGitRepo, false)
			if err != nil {
				slog.Debug("worktree list failed for status enrichment", "container", projects[idx].ID, "err", err)
				projects[idx].AgentStatus = ec.checkAgentStatus(ctx, projects[idx].ID)
				return
			}

			activeCount := 0
			for _, wt := range worktrees {
				if wt.State == WorktreeStateConnected {
					activeCount++
				}
			}

			projects[idx].ActiveWorktreeCount = activeCount
			// Attention state (NeedsInput, NotificationType) is push-based
			// via the event bus, not derivable from container inspection.
			// The service layer overlays it from the event store.

			if activeCount > 0 {
				projects[idx].AgentStatus = AgentStatusWorking
			} else {
				// Fall back to pgrep for cases where Claude was started
				// manually outside of Warden's terminal management
				projects[idx].AgentStatus = ec.checkAgentStatus(ctx, projects[idx].ID)
			}
		}(i)
	}

	wg.Wait()
}

// checkAgentStatus runs pgrep inside the container to detect a running agent process.
// Uses the provider's ProcessName for the correct binary to search for.
// Uses -x for exact process name match to avoid false positives.
func (ec *EngineClient) checkAgentStatus(ctx context.Context, containerID string) AgentStatus {
	provider := ec.resolveProvider(ctx, containerID)
	processName := "claude"
	if provider != nil {
		processName = provider.ProcessName()
	}

	execCfg := container.ExecOptions{
		Cmd:          []string{"pgrep", "-x", processName},
		AttachStdout: true,
		AttachStderr: true,
	}

	output, err := ec.execAndCapture(ctx, containerID, execCfg)
	if err != nil {
		slog.Debug("agent status check failed", "container", containerID, "err", err)
		return AgentStatusUnknown
	}

	if strings.TrimSpace(output) != "" {
		return AgentStatusWorking
	}
	return AgentStatusIdle
}

// workspaceDir resolves the container-side workspace directory for a container.
// Reads WARDEN_WORKSPACE_DIR from the container's env vars (set at creation).
// Falls back to scanning bind mounts, then /project for legacy containers.
// The result is cached per container ID.
func (ec *EngineClient) workspaceDir(ctx context.Context, containerID string) string {
	if cached, ok := ec.workspaceDirCache.Load(containerID); ok {
		return cached.(string)
	}

	dir := ec.resolveWorkspaceDir(ctx, containerID)
	ec.workspaceDirCache.Store(containerID, dir)
	return dir
}

// resolveWorkspaceDir inspects a container to find its workspace directory.
func (ec *EngineClient) resolveWorkspaceDir(ctx context.Context, containerID string) string {
	info, err := ec.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return "/project" // Fallback for legacy containers.
	}

	// Check WARDEN_WORKSPACE_DIR env var (set by Warden at creation).
	if wsDir := envValue(info.Config.Env, "WARDEN_WORKSPACE_DIR"); wsDir != "" {
		return wsDir
	}

	// Fallback for discovered/legacy containers: find the workspace bind mount.
	// Check for /home/warden/<name> pattern first, then /project.
	// Checks both HostConfig.Binds (Docker) and Mounts (Podman) since
	// Podman may populate only the Mounts field.
	name := strings.TrimPrefix(info.Name, "/")
	expectedPath := ContainerWorkspaceDir(name)

	if info.HostConfig != nil {
		for _, bind := range info.HostConfig.Binds {
			parts := strings.SplitN(bind, ":", 2)
			if len(parts) < 2 {
				continue
			}
			containerPath, _, _ := strings.Cut(parts[1], ":")
			if containerPath == expectedPath {
				return expectedPath
			}
		}
		for _, bind := range info.HostConfig.Binds {
			parts := strings.SplitN(bind, ":", 2)
			if len(parts) < 2 {
				continue
			}
			containerPath, _, _ := strings.Cut(parts[1], ":")
			if containerPath == "/project" {
				return "/project"
			}
		}
	}

	// Podman populates Mounts instead of HostConfig.Binds.
	for _, m := range info.Mounts {
		if m.Destination == expectedPath {
			return expectedPath
		}
	}
	for _, m := range info.Mounts {
		if m.Destination == "/project" {
			return "/project"
		}
	}

	return "/project"
}

// checkIsGitRepo checks whether the workspace is a git repository inside the container.
// The result is cached per container ID since this value is effectively static for
// the lifetime of a running container.
func (ec *EngineClient) checkIsGitRepo(ctx context.Context, containerID string) bool {
	if cached, ok := ec.gitRepoCache.Load(containerID); ok {
		return cached.(bool)
	}

	wsDir := ec.workspaceDir(ctx, containerID)
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"git", "-C", wsDir, "-c", "safe.directory=" + wsDir, "rev-parse", "--is-inside-work-tree"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return false
	}

	result := strings.TrimSpace(output) == "true"
	ec.gitRepoCache.Store(containerID, result)
	return result
}

// envValue extracts the value of a KEY=value pair from an env slice.
// Returns "" if the key is not found.
func envValue(envs []string, key string) string {
	prefix := key + "="
	for _, e := range envs {
		if strings.HasPrefix(e, prefix) {
			return strings.TrimPrefix(e, prefix)
		}
	}
	return ""
}

// containerName extracts the primary container name, stripping Docker's leading slash.
func containerName(names []string) string {
	if len(names) == 0 {
		return ""
	}
	return strings.TrimPrefix(names[0], "/")
}

// containerToProject converts a Docker container summary into a Project.
// Only Docker-derived fields are populated here. Project metadata
// (skipPermissions, networkMode, allowedDomains) is overlaid from the
// database by the service layer.
func containerToProject(c container.Summary) Project {
	id := c.ID
	if len(id) > 12 {
		id = id[:12]
	}

	name := containerName(c.Names)
	mountSource, mountDest := projectMountPaths(name, c.Mounts)

	return Project{
		ID:           id,
		Name:         name,
		HasContainer: true,
		HostPath:     mountSource,
		Type:         c.Labels["project.type"],
		Image:        truncateImage(c.Image),
		OS:           buildOSLabel(c.Labels),
		CreatedAt:    c.Created,
		SSHPort:      c.Labels["project.ssh.port"],
		State:        c.State,
		Status:       c.Status,
		MountedDir:   mountSource,
		WorkspaceDir: mountDest,
	}
}

// projectMountPaths returns the host path (source) and container path (destination)
// of the workspace bind mount. Checks for WARDEN_WORKSPACE_DIR-style mounts under
// /home/warden/ first, then falls back to the legacy /project mount.
func projectMountPaths(name string, mounts []container.MountPoint) (source, destination string) {
	expected := ContainerWorkspaceDir(name)
	for _, m := range mounts {
		if m.Destination == expected {
			return m.Source, m.Destination
		}
	}
	// Legacy fallback.
	for _, m := range mounts {
		if m.Destination == "/project" {
			return m.Source, m.Destination
		}
	}
	return "", ""
}

// truncateImage shortens a sha256 image digest to "sha256:abcdef012345" (12 hex chars).
// Named image references (e.g. "ubuntu:24.04") are returned unchanged.
func truncateImage(image string) string {
	const prefix = "sha256:"
	if after, ok := strings.CutPrefix(image, prefix); ok && len(after) > 12 {
		return prefix + after[:12]
	}
	return image
}

// buildOSLabel combines the OCI image name and version labels into a human-readable string.
// Returns an empty string if neither label is present.
func buildOSLabel(labels map[string]string) string {
	name := labels["org.opencontainers.image.ref.name"]
	version := labels["org.opencontainers.image.version"]
	switch {
	case name != "" && version != "":
		return name + " " + version
	case name != "":
		return name
	case version != "":
		return version
	default:
		return ""
	}
}

// findHostPort returns the first host port mapped to the given container port.
func findHostPort(ports []container.Port, containerPort uint16) string {
	for _, p := range ports {
		if p.PrivatePort == containerPort && p.PublicPort != 0 {
			return fmt.Sprintf("%d", p.PublicPort)
		}
	}
	return ""
}

// execAndCapture runs an exec command and returns its demuxed stdout as a string.
// Docker wraps exec output in a multiplexed stream; stdcopy strips the framing.
func (ec *EngineClient) execAndCapture(ctx context.Context, containerID string, cfg container.ExecOptions) (string, error) {
	resp, err := ec.api.ContainerExecCreate(ctx, containerID, cfg)
	if err != nil {
		return "", fmt.Errorf("creating exec: %w", err)
	}

	hijacked, err := ec.api.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("attaching to exec: %w", err)
	}
	defer hijacked.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, hijacked.Reader); err != nil {
		return "", fmt.Errorf("reading exec output: %w", err)
	}

	return stdout.String(), nil
}

// execAndCaptureStrict runs an exec command, returns its stdout, and fails if the
// command exits with a non-zero status. Use this for scripts that must succeed
// (e.g. create-terminal.sh). Use execAndCapture for commands where non-zero exits
// are expected (e.g. pgrep, git rev-parse on non-git repos).
func (ec *EngineClient) execAndCaptureStrict(ctx context.Context, containerID string, cfg container.ExecOptions) (string, error) {
	resp, err := ec.api.ContainerExecCreate(ctx, containerID, cfg)
	if err != nil {
		return "", fmt.Errorf("creating exec: %w", err)
	}

	hijacked, err := ec.api.ContainerExecAttach(ctx, resp.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", fmt.Errorf("attaching to exec: %w", err)
	}
	defer hijacked.Close()

	var stdout, stderr bytes.Buffer
	if _, err := stdcopy.StdCopy(&stdout, &stderr, hijacked.Reader); err != nil {
		return "", fmt.Errorf("reading exec output: %w", err)
	}

	inspect, err := ec.api.ContainerExecInspect(ctx, resp.ID)
	if err != nil {
		return "", fmt.Errorf("inspecting exec result: %w", err)
	}
	if inspect.ExitCode != 0 {
		return "", fmt.Errorf("command exited with status %d: %s", inspect.ExitCode, strings.TrimSpace(stderr.String()))
	}

	return stdout.String(), nil
}

// requiredBinaries lists the executables that must be present inside a container
// for Warden's terminal infrastructure to work.
var requiredBinaries = []string{
	"/usr/local/bin/gosu",
	"/usr/local/bin/entrypoint.sh",
	"/usr/local/bin/user-entrypoint.sh",
	"/usr/local/bin/create-terminal.sh",
	"/usr/local/bin/disconnect-terminal.sh",
	"/usr/local/bin/kill-worktree.sh",
}

// ValidateInfrastructure checks whether a container has the required Warden
// terminal infrastructure installed. Uses POSIX `test -x` to check each binary
// so it works in minimal containers without `which`.
func (ec *EngineClient) ValidateInfrastructure(ctx context.Context, containerID string) (bool, []string, error) {
	// Build a single command that tests all binaries and reports missing ones
	var checks []string
	for _, bin := range requiredBinaries {
		checks = append(checks, fmt.Sprintf(`test -x %s || echo %s`, bin, bin))
	}

	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", strings.Join(checks, " ; ")},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return false, nil, fmt.Errorf("validating infrastructure: %w", err)
	}

	var missing []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			missing = append(missing, line)
		}
	}

	return len(missing) == 0, missing, nil
}

// StopProject gracefully stops a container with a 30-second timeout.
func (ec *EngineClient) StopProject(ctx context.Context, id string) error {
	timeout := int(stopTimeout.Seconds())
	err := ec.api.ContainerStop(ctx, id, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("stopping container %s: %w", id, err)
	}

	slog.Info("stopped project", "id", id)
	return nil
}

// RestartProject restarts a container. Before restarting, it validates
// that all bind mount source paths still exist on the host. If any are
// stale (e.g. Nix Home Manager switched generations and garbage-collected
// old store paths), the restart is blocked with a StaleMountsError so the
// caller can warn the user and let them decide how to proceed.
//
// originalMounts are the pre-symlink-resolution mount specs from the DB.
// When nil, mount validation is skipped (container predates the migration).
func (ec *EngineClient) RestartProject(ctx context.Context, id string, originalMounts []Mount) error {
	if err := ec.validateMountSources(ctx, id, originalMounts); err != nil {
		return err
	}

	timeout := int(stopTimeout.Seconds())
	err := ec.api.ContainerRestart(ctx, id, container.StopOptions{
		Timeout: &timeout,
	})
	if err != nil {
		return fmt.Errorf("restarting container %s: %w", id, err)
	}

	slog.Info("restarted project", "id", id)
	return nil
}

// validateMountSources re-resolves the original mount specs and compares
// them with the container's current bind mounts. If the resolution has
// changed (symlink targets moved, deleted, or new symlinks appeared),
// returns a StaleMountsError to block the restart.
//
// originalMounts come from the DB. When nil, validation is skipped.
func (ec *EngineClient) validateMountSources(ctx context.Context, id string, originalMounts []Mount) error {
	if len(originalMounts) == 0 {
		return nil
	}

	info, err := ec.api.ContainerInspect(ctx, id)
	if err != nil {
		return fmt.Errorf("inspecting container for mount validation: %w", err)
	}

	// Parse current binds into mounts for comparison.
	// Check both HostConfig.Binds (Docker) and Mounts (Podman).
	// Skip Warden-managed mounts (workspace dir, event dir) — these are
	// created fresh by Warden and should not be compared against the
	// user-configured original mounts stored in the DB.
	wsDir := envValue(info.Config.Env, "WARDEN_WORKSPACE_DIR")
	isWardenManaged := func(containerPath string) bool {
		return containerPath == wsDir || containerPath == containerEventDir
	}

	var currentMounts []Mount
	if info.HostConfig != nil {
		for _, bind := range info.HostConfig.Binds {
			parts := strings.SplitN(bind, ":", 2)
			if len(parts) != 2 {
				continue
			}
			hostPath := parts[0]
			remainder := parts[1]
			containerPath, _, _ := strings.Cut(remainder, ":")

			if isWardenManaged(containerPath) {
				continue
			}
			currentMounts = append(currentMounts, Mount{
				HostPath:      hostPath,
				ContainerPath: containerPath,
			})
		}
	}

	// Podman populates Mounts instead of HostConfig.Binds.
	if len(currentMounts) == 0 {
		for _, m := range info.Mounts {
			if isWardenManaged(m.Destination) {
				continue
			}
			currentMounts = append(currentMounts, Mount{
				HostPath:      m.Source,
				ContainerPath: m.Destination,
			})
		}
	}

	stalePaths := DetectStaleMounts(originalMounts, currentMounts)
	if len(stalePaths) == 0 {
		return nil
	}

	for _, p := range stalePaths {
		slog.Warn("stale bind mount detected", "containerPath", p)
	}

	return &StaleMountsError{StalePaths: stalePaths}
}

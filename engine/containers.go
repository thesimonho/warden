package engine

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
)

// defaultPidsLimit is the maximum number of processes allowed in a container.
// Prevents fork bombs while leaving ample headroom for Claude Code + dev tools.
const defaultPidsLimit = int64(512)

// defaultImage is the container image used when none is specified.
const defaultImage = "ghcr.io/thesimonho/warden:latest"

// containerEventDir is the in-container path where event files are written.
const containerEventDir = "/var/warden/events"

// cacheVolumeName is the Docker named volume for persisting CLI downloads
// and language runtime package caches across container recreates.
const cacheVolumeName = "warden-cache"

// cacheVolumeTarget is the in-container mount point for the cache volume.
const cacheVolumeTarget = "/home/warden/.cache/warden-runtimes"

// ErrNameTaken is returned when a container with the requested name already exists.
var ErrNameTaken = fmt.Errorf("container name already in use")

// checkNameAvailable returns ErrNameTaken if a container with the given name already exists.
func (ec *EngineClient) checkNameAvailable(ctx context.Context, name string) error {
	_, err := ec.api.ContainerInspect(ctx, name)
	if err != nil {
		return nil // container doesn't exist, name is available
	}
	return fmt.Errorf("%w: %q — remove the existing container first, or choose a different name", ErrNameTaken, name)
}

// ensureImage pulls the container image if it is not already available locally.
func (ec *EngineClient) ensureImage(ctx context.Context, imageName string) error {
	_, err := ec.api.ImageInspect(ctx, imageName)
	if err == nil {
		return nil
	}

	slog.Info("pulling image", "image", imageName)
	reader, err := ec.api.ImagePull(ctx, imageName, image.PullOptions{})
	if err != nil {
		return fmt.Errorf("pulling image %q: %w", imageName, err)
	}
	defer reader.Close() //nolint:errcheck

	// Consume the pull output to completion.
	if _, err := io.Copy(io.Discard, reader); err != nil {
		return fmt.Errorf("reading pull response for %q: %w", imageName, err)
	}

	slog.Info("pulled image", "image", imageName)
	return nil
}

// CreateContainer creates and starts a new project container with the
// given configuration. Returns the container ID (truncated to 12 chars).
func (ec *EngineClient) CreateContainer(ctx context.Context, req api.CreateContainerRequest) (string, error) {
	if req.Name == "" {
		return "", fmt.Errorf("container name is required")
	}
	if req.ProjectPath == "" {
		return "", fmt.Errorf("project path is required")
	}
	if !filepath.IsAbs(req.ProjectPath) {
		return "", fmt.Errorf("project path must be absolute: %s", req.ProjectPath)
	}

	// Stat the project path early to fail fast before expensive operations
	// like image pulls. The entrypoint uses the host UID/GID to match
	// file ownership via usermod before exec-ing gosu.
	hostUID, hostGID, err := hostOwner(req.ProjectPath)
	if err != nil {
		return "", fmt.Errorf("stat project path for UID/GID: %w", err)
	}

	if err := ec.checkNameAvailable(ctx, req.Name); err != nil {
		return "", err
	}

	image := req.Image
	if image == "" {
		image = defaultImage
	}

	if err := ec.ensureImage(ctx, image); err != nil {
		return "", err
	}

	// Build env vars list
	envList := make([]string, 0, len(req.EnvVars))
	for k, v := range req.EnvVars {
		envList = append(envList, fmt.Sprintf("%s=%s", k, v))
	}

	// Default to full network access when unset
	networkMode := req.NetworkMode
	if networkMode == "" {
		networkMode = api.NetworkModeFull
	}

	// Label Warden-managed containers so the Docker events watcher can filter
	// container start events efficiently (re-apply network isolation on restart).
	labels := map[string]string{
		"dev.warden.managed": "true",
	}

	// Pass network mode to the container as env vars so the entrypoint
	// can set up iptables rules for restricted/none modes.
	envList = append(envList, fmt.Sprintf("WARDEN_NETWORK_MODE=%s", networkMode))
	if networkMode == api.NetworkModeRestricted && len(req.AllowedDomains) > 0 {
		envList = append(envList, fmt.Sprintf("WARDEN_ALLOWED_DOMAINS=%s", strings.Join(req.AllowedDomains, ",")))
	}

	// Pass container name and project ID so hook scripts can identify
	// this container and its stable project identity in event payloads.
	envList = append(envList, fmt.Sprintf("WARDEN_CONTAINER_NAME=%s", req.Name))
	if projectID, err := ProjectID(req.ProjectPath); err == nil {
		envList = append(envList, fmt.Sprintf("WARDEN_PROJECT_ID=%s", projectID))
	}

	// Pass the host UID/GID so the entrypoint can match file ownership
	// without probing bind mounts at runtime.
	envList = append(envList,
		fmt.Sprintf("WARDEN_HOST_UID=%d", hostUID),
		fmt.Sprintf("WARDEN_HOST_GID=%d", hostGID),
	)

	// Set the agent type so container scripts know which CLI to launch.
	// The service layer defaults this to agent.DefaultAgentType before calling.
	envList = append(envList, fmt.Sprintf("WARDEN_AGENT_TYPE=%s", req.AgentType))

	// Set the workspace directory inside the container. Each project gets
	// a unique path (/home/warden/<name>) so the agent's config file keys
	// don't collide across containers (they share the file via bind mount).
	containerWSDir := ContainerWorkspaceDir(req.Name)
	envList = append(envList, fmt.Sprintf("WARDEN_WORKSPACE_DIR=%s", containerWSDir))

	// Tell container hooks where to write event files. The host-side
	// directory is bind-mounted at containerEventDir inside the container.
	envList = append(envList, fmt.Sprintf("WARDEN_EVENT_DIR=%s", containerEventDir))

	// Pass pinned agent CLI versions so the entrypoint installs the exact
	// version validated by the parser. Only the relevant version is used
	// (based on WARDEN_AGENT_TYPE), but both are passed for simplicity.
	envList = append(envList,
		fmt.Sprintf("WARDEN_CLAUDE_VERSION=%s", agent.ClaudeCodeVersion),
		fmt.Sprintf("WARDEN_CODEX_VERSION=%s", agent.CodexVersion),
	)

	// Pass enabled runtimes so the entrypoint can install them.
	if len(req.EnabledRuntimes) > 0 {
		envList = append(envList, fmt.Sprintf("WARDEN_ENABLED_RUNTIMES=%s", strings.Join(req.EnabledRuntimes, ",")))
	}

	containerConfig := &container.Config{
		Image:      image,
		Env:        envList,
		Labels:     labels,
		Hostname:   req.Name,
		Entrypoint: []string{"/usr/local/bin/entrypoint.sh"},
	}

	resolvedMounts, err := resolveSymlinksForMounts(req.Mounts)
	if err != nil {
		return "", fmt.Errorf("resolving symlinks in mounts: %w", err)
	}

	binds, err := buildBindMounts(req.ProjectPath, containerWSDir, resolvedMounts)
	if err != nil {
		return "", err
	}

	// Create and bind-mount the event directory for file-based IPC.
	if ec.eventBaseDir != "" {
		eventHostDir := filepath.Join(ec.eventBaseDir, req.Name)
		if mkErr := os.MkdirAll(eventHostDir, 0o777); mkErr != nil {
			return "", fmt.Errorf("creating event directory: %w", mkErr)
		}
		// Explicit chmod because MkdirAll is affected by umask.
		if chmodErr := os.Chmod(eventHostDir, 0o777); chmodErr != nil {
			// Directory may be owned by a different user (e.g. leftover from a
			// different runtime). Remove the container-level dir and recreate.
			containerDir := filepath.Join(ec.eventBaseDir, req.Name)
			if rmErr := os.RemoveAll(containerDir); rmErr != nil {
				return "", fmt.Errorf(
					"event directory %q has wrong ownership and could not be cleaned up: %w",
					containerDir, rmErr,
				)
			}
			if mkErr := os.MkdirAll(eventHostDir, 0o777); mkErr != nil {
				return "", fmt.Errorf("recreating event directory: %w", mkErr)
			}
			if retryErr := os.Chmod(eventHostDir, 0o777); retryErr != nil {
				return "", fmt.Errorf("setting event directory permissions after recreate: %w", retryErr)
			}
		}
		binds = append(binds, fmt.Sprintf("%s:%s", eventHostDir, containerEventDir))
	}

	capDrop, capAdd, securityOpts := buildSecurityConfig(ec.seccompProfileJSON)

	pidsLimit := defaultPidsLimit
	hostConfig := &container.HostConfig{
		Binds: binds,
		RestartPolicy: container.RestartPolicy{
			Name: container.RestartPolicyUnlessStopped,
		},
		Resources: container.Resources{
			PidsLimit: &pidsLimit,
		},
		// Keep host.docker.internal mapping — harmless and may be useful
		// for user tools inside the container.
		ExtraHosts:  []string{"host.docker.internal:host-gateway"},
		CapDrop:     capDrop,
		CapAdd:      capAdd,
		SecurityOpt: securityOpts,
	}

	// Mount the shared cache volume unconditionally — the agent CLI
	// installer caches downloaded binaries here, and language runtimes
	// use it for package caches (Go modules, Cargo, pip, etc.).
	hostConfig.Mounts = []mount.Mount{
		{Type: mount.TypeVolume, Source: cacheVolumeName, Target: cacheVolumeTarget},
	}

	resp, err := ec.api.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, req.Name)
	if err != nil {
		return "", fmt.Errorf("creating container %q: %w", req.Name, err)
	}

	if err := ec.api.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		// Remove the created-but-not-started container so it doesn't block
		// future attempts with an ErrNameTaken error.
		_ = ec.api.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true})
		return "", fmt.Errorf("starting container %q: %w", req.Name, err)
	}

	id := resp.ID
	if len(id) > 12 {
		id = id[:12]
	}

	// Apply network isolation via privileged docker exec. This runs the
	// same setup-network-isolation.sh script but from outside the container,
	// so NET_ADMIN is never in the container's bounding set. Even sudo
	// can't undo iptables rules without NET_ADMIN.
	if networkMode != api.NetworkModeFull {
		if err := ec.ApplyNetworkIsolation(ctx, id, string(networkMode), req.AllowedDomains); err != nil {
			_ = ec.stopAndRemove(ctx, resp.ID)
			return "", fmt.Errorf("setting up network isolation: %w", err)
		}
	}

	slog.Info("created container", "name", req.Name, "id", id, "image", image)
	return id, nil
}

// stopAndRemove stops a container (ignoring already-stopped errors) and force-removes it,
// clearing its git repo cache entry.
func (ec *EngineClient) stopAndRemove(ctx context.Context, id string) error {
	timeout := int(stopTimeout.Seconds())
	_ = ec.api.ContainerStop(ctx, id, container.StopOptions{Timeout: &timeout})

	if err := ec.api.ContainerRemove(ctx, id, container.RemoveOptions{Force: true}); err != nil {
		return fmt.Errorf("removing container %s: %w", id, err)
	}

	ec.gitRepoCache.Delete(id)
	return nil
}

// DeleteContainer stops and removes a container, clearing its git repo cache entry.
func (ec *EngineClient) DeleteContainer(ctx context.Context, id string) error {
	if err := ec.stopAndRemove(ctx, id); err != nil {
		return err
	}
	slog.Info("deleted container", "id", id)
	return nil
}

// InspectContainer returns the editable configuration of an existing container
// by parsing its inspect data (binds, env vars, labels).
func (ec *EngineClient) InspectContainer(ctx context.Context, id string) (*api.ContainerConfig, error) {
	info, err := ec.api.ContainerInspect(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("inspecting container %s: %w", id, err)
	}

	cfg := &api.ContainerConfig{
		Name:  strings.TrimPrefix(info.Name, "/"),
		Image: info.Config.Image,
	}

	// Read agent type from the container's env.
	cfg.AgentType = constants.AgentType(envValue(info.Config.Env, "WARDEN_AGENT_TYPE"))
	if cfg.AgentType == "" {
		cfg.AgentType = agent.DefaultType
	}

	// Determine the workspace mount path from env vars.
	wsDir := envValue(info.Config.Env, "WARDEN_WORKSPACE_DIR")
	// Fallback for legacy/discovered containers.
	if wsDir == "" {
		wsDir = ContainerWorkspaceDir(cfg.Name)
	}

	// Parse binds for project path and additional mounts.
	if info.HostConfig != nil {
		for _, bind := range info.HostConfig.Binds {
			parts := strings.SplitN(bind, ":", 2)
			if len(parts) != 2 {
				continue
			}
			hostPath := parts[0]
			remainder := parts[1]
			// Remainder may include :ro suffix
			containerPath, suffix, _ := strings.Cut(remainder, ":")
			readOnly := suffix == "ro"

			if containerPath == wsDir || containerPath == "/project" {
				cfg.ProjectPath = hostPath
			} else {
				cfg.Mounts = append(cfg.Mounts, api.Mount{
					HostPath:      hostPath,
					ContainerPath: containerPath,
					ReadOnly:      readOnly,
				})
			}
		}
	}

	// Fallback: check Mounts field for legacy/discovered containers.
	if cfg.ProjectPath == "" {
		for _, m := range info.Mounts {
			if m.Destination == wsDir || m.Destination == "/project" {
				cfg.ProjectPath = m.Source
			} else {
				cfg.Mounts = append(cfg.Mounts, api.Mount{
					HostPath:      m.Source,
					ContainerPath: m.Destination,
					ReadOnly:      !m.RW,
				})
			}
		}
	}

	// Parse env vars, filtering out system-injected and warden-internal ones
	systemEnvPrefixes := []string{"PATH=", "HOME=", "HOSTNAME=", "TERM=", "WARDEN_"}
	envMap := make(map[string]string)
	for _, env := range info.Config.Env {
		isSystem := false
		for _, prefix := range systemEnvPrefixes {
			if strings.HasPrefix(env, prefix) {
				isSystem = true
				break
			}
		}
		if isSystem {
			continue
		}
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envMap[parts[0]] = parts[1]
		}
	}
	if len(envMap) > 0 {
		cfg.EnvVars = envMap
	}

	return cfg, nil
}

// ContainerIP returns the bridge network IP address of a running container.
// Returns an error if the container has no bridge network or the IP is empty
// (e.g. container is stopped).
func (ec *EngineClient) ContainerIP(ctx context.Context, containerID string) (string, error) {
	info, err := ec.api.ContainerInspect(ctx, containerID)
	if err != nil {
		return "", fmt.Errorf("inspecting container %s: %w", containerID, err)
	}
	if info.NetworkSettings == nil || info.NetworkSettings.Networks == nil {
		return "", fmt.Errorf("container %s has no network settings", containerID)
	}
	bridge, ok := info.NetworkSettings.Networks["bridge"]
	if !ok || bridge.IPAddress == "" {
		return "", fmt.Errorf("container %s has no bridge network IP", containerID)
	}
	return bridge.IPAddress, nil
}

// RenameContainer changes the name of an existing container without recreation.
func (ec *EngineClient) RenameContainer(ctx context.Context, id string, newName string) error {
	return ec.api.ContainerRename(ctx, id, newName)
}

// ReloadAllowedDomains re-runs the network isolation script inside a running
// container to update the allowed domain list without recreation. Delegates
// to ApplyNetworkIsolation with restricted mode hardcoded (the only mode
// that uses domain hot-reload).
func (ec *EngineClient) ReloadAllowedDomains(ctx context.Context, containerID string, domains []string) error {
	if err := ec.ApplyNetworkIsolation(ctx, containerID, string(api.NetworkModeRestricted), domains); err != nil {
		return fmt.Errorf("reloading allowed domains: %w", err)
	}
	return nil
}

// ApplyNetworkIsolation runs the network isolation script via privileged
// docker exec. Used after container start/restart to set up iptables
// without granting NET_ADMIN to the container's capability set. This makes
// network isolation tamper-proof — even root inside the container cannot
// modify iptables rules.
func (ec *EngineClient) ApplyNetworkIsolation(ctx context.Context, containerID, mode string, domains []string) error {
	cfg := container.ExecOptions{
		Cmd:          []string{"/usr/local/bin/setup-network-isolation.sh"},
		User:         "root",
		Privileged:   true,
		Env:          []string{"WARDEN_NETWORK_MODE=" + mode, "WARDEN_ALLOWED_DOMAINS=" + strings.Join(domains, ",")},
		AttachStdout: true,
		AttachStderr: true,
	}
	_, err := ec.execAndCaptureStrict(ctx, containerID, cfg)
	if err != nil {
		return fmt.Errorf("applying network isolation: %w", err)
	}
	return nil
}

// RecreateContainer replaces a stopped container with a new one using updated config.
// The old container is renamed to a temporary name before creating the replacement,
// so it can be restored if the create fails (atomic swap).
func (ec *EngineClient) RecreateContainer(ctx context.Context, id string, req api.CreateContainerRequest) (string, error) {
	info, err := ec.api.ContainerInspect(ctx, id)
	if err != nil {
		return "", fmt.Errorf("inspecting container for recreate: %w", err)
	}

	oldName := strings.TrimPrefix(info.Name, "/")
	if req.Name == "" {
		req.Name = oldName
	}

	// Rename the old container to free up the name for the replacement.
	// If anything fails after this point, we rename it back.
	tempName := oldName + "-warden-replacing"
	if err := ec.api.ContainerRename(ctx, id, tempName); err != nil {
		return "", fmt.Errorf("renaming old container: %w", err)
	}

	newID, err := ec.CreateContainer(ctx, req)
	if err != nil {
		// Restore the old container's name so the user isn't left with nothing.
		if renameErr := ec.api.ContainerRename(ctx, id, oldName); renameErr != nil {
			slog.Error("failed to restore old container name after failed recreate",
				"id", id, "tempName", tempName, "err", renameErr)
		}
		return "", fmt.Errorf("creating replacement container: %w", err)
	}

	// New container created successfully — remove the old one.
	if err := ec.stopAndRemove(ctx, id); err != nil {
		slog.Warn("replacement created but failed to remove old container",
			"oldId", id, "newId", newID, "err", err)
	}

	slog.Info("recreated container", "oldId", id, "newId", newID, "name", req.Name)
	return newID, nil
}

// baseCapabilities are the Linux capabilities granted to every Warden container.
// These are the minimum set required for the entrypoint (root → warden user switch
// via gosu, chown, kill), sudo for package installation, and standard dev tooling
// (bind to low ports, ping).
//
// Dropped from Docker's defaults: SETPCAP, MKNOD, SETFCAP, AUDIT_WRITE — these
// allow modifying capability sets, creating device nodes, and writing to the
// kernel audit log, none of which is needed in a coding agent container.
//
// Notably absent: NET_ADMIN. Network isolation (iptables) is applied via
// privileged docker exec from the Go server, keeping it out of the bounding
// set so even root-via-sudo cannot modify firewall rules.
var baseCapabilities = []string{
	"CHOWN",            // entrypoint chown of bind mounts
	"DAC_OVERRIDE",     // root reading/writing files owned by warden user
	"FOWNER",           // entrypoint file ownership operations
	"FSETID",           // preserve setuid/setgid bits during chown
	"KILL",             // shutdown handler: kill -TERM -1
	"SETUID",           // gosu privilege drop (setuid syscall from root)
	"SETGID",           // gosu privilege drop (setgid syscall from root)
	"NET_BIND_SERVICE", // dev servers binding to ports < 1024
	"NET_RAW",          // ping and network diagnostics
	"SYS_CHROOT",       // some tools (e.g. npm) use chroot for sandboxing
}

// buildSecurityConfig returns the capability drop/add lists and security
// options for a container. Every container gets the same security profile:
//   - CapDrop ALL (drop all default capabilities)
//   - CapAdd with baseCapabilities (re-add only what's needed)
//   - Custom seccomp profile (denylist of dangerous syscalls) as inline JSON
//
// NET_ADMIN is intentionally excluded from all modes. Network isolation
// (iptables/dnsmasq) is applied via privileged docker exec from the Go
// server after container start. This keeps NET_ADMIN out of the container's
// bounding set, making network rules tamper-proof — even sudo cannot
// modify iptables without NET_ADMIN.
//
// no-new-privileges is not set so that sudo (a SUID binary) can elevate
// to root for package installation. This is safe because the bounding set
// is tight: no NET_ADMIN (can't touch iptables), no SYS_ADMIN (can't
// mount/unmount), no MKNOD (can't create devices).
func buildSecurityConfig(seccompValue string) (capDrop, capAdd []string, securityOpts []string) {
	capDrop = []string{"ALL"}

	capAdd = make([]string, len(baseCapabilities))
	copy(capAdd, baseCapabilities)

	securityOpts = []string{
		"seccomp=" + seccompValue,
	}

	return capDrop, capAdd, securityOpts
}

// PreWarmCLICache downloads pinned agent CLIs into the warden-cache volume
// using throwaway containers. Both agent types run in parallel. Subsequent
// container creates get a cache hit and skip the download.
func (ec *EngineClient) PreWarmCLICache(ctx context.Context) error {
	img := defaultImage
	if err := ec.ensureImage(ctx, img); err != nil {
		return fmt.Errorf("ensuring image for pre-warm: %w", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, 2)

	for i, agentType := range []string{"claude-code", "codex"} {
		wg.Add(1)
		go func(idx int, at string) {
			defer wg.Done()
			errs[idx] = ec.runEphemeralInstall(ctx, img, at)
		}(i, agentType)
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	slog.Info("CLI cache pre-warmed", "claudeVersion", agent.ClaudeCodeVersion, "codexVersion", agent.CodexVersion)
	return nil
}

// runEphemeralInstall creates a throwaway container that runs install-agent.sh
// for the given agent type, waits for completion, then removes the container.
func (ec *EngineClient) runEphemeralInstall(ctx context.Context, img, agentType string) error {
	resp, err := ec.api.ContainerCreate(ctx, &container.Config{
		Image: img,
		Env: []string{
			fmt.Sprintf("WARDEN_AGENT_TYPE=%s", agentType),
			fmt.Sprintf("WARDEN_CLAUDE_VERSION=%s", agent.ClaudeCodeVersion),
			fmt.Sprintf("WARDEN_CODEX_VERSION=%s", agent.CodexVersion),
		},
		Entrypoint: []string{"/usr/local/bin/install-agent.sh"},
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{Type: mount.TypeVolume, Source: cacheVolumeName, Target: cacheVolumeTarget},
		},
	}, nil, nil, "")
	if err != nil {
		return fmt.Errorf("creating pre-warm %s container: %w", agentType, err)
	}
	defer func() { _ = ec.api.ContainerRemove(ctx, resp.ID, container.RemoveOptions{Force: true}) }()

	if err := ec.api.ContainerStart(ctx, resp.ID, container.StartOptions{}); err != nil {
		return fmt.Errorf("starting pre-warm %s container: %w", agentType, err)
	}

	waitCh, errCh := ec.api.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case <-waitCh:
	case err := <-errCh:
		if err != nil {
			slog.Warn("pre-warm wait error", "agentType", agentType, "err", err)
		}
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}

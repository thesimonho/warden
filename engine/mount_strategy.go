package engine

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/mount"

	"github.com/thesimonho/warden/api"
)

// buildBindMounts constructs the bind mount strings for container creation.
// For local projects, the host directory is mounted at containerWorkspaceDir
// (typically /home/warden/<name>). For remote projects, projectPath is empty
// and the workspace mount is skipped. Additional mounts are appended with
// optional :ro suffix.
func buildBindMounts(projectPath, containerWorkspaceDir string, mounts []api.Mount) ([]string, error) {
	var binds []string
	if projectPath != "" {
		binds = append(binds, fmt.Sprintf("%s:%s", projectPath, containerWorkspaceDir))
	}

	for _, m := range mounts {
		if !filepath.IsAbs(m.HostPath) {
			return nil, fmt.Errorf("mount host path must be absolute: %s", m.HostPath)
		}
		if !filepath.IsAbs(m.ContainerPath) {
			return nil, fmt.Errorf("mount container path must be absolute: %s", m.ContainerPath)
		}
		bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
		if m.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	return binds, nil
}

// buildSocketMounts converts socket mounts (e.g. SSH agent) into Docker
// structured mount.Mount entries. These use the Docker mount API instead
// of legacy Binds strings because Binds auto-creates missing host paths
// as directories, which fails for sockets on macOS with Docker Desktop.
func buildSocketMounts(mounts []api.Mount) []mount.Mount {
	if len(mounts) == 0 {
		return nil
	}
	result := make([]mount.Mount, 0, len(mounts))
	for _, m := range mounts {
		result = append(result, mount.Mount{
			Type:     mount.TypeBind,
			Source:   m.HostPath,
			Target:   m.ContainerPath,
			ReadOnly: m.ReadOnly,
		})
	}
	return result
}

// findFailingMount returns the index of the mount whose source path appears
// in the Docker error message, or -1 if no match is found. Docker errors
// typically include the path verbatim (e.g. "bind source path does not
// exist: /run/user/1000/gnupg/S.gpg-agent").
func findFailingMount(err error, mounts []mount.Mount) int {
	msg := err.Error()
	for i, m := range mounts {
		if m.Source != "" && strings.Contains(msg, m.Source) {
			return i
		}
	}
	return -1
}

// isMountError reports whether a Docker API error is related to a bind mount
// failure (e.g. source path doesn't exist or can't be stat'd). Used to narrow
// the socket mount retry to mount-specific failures.
func isMountError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "mount source") ||
		strings.Contains(msg, "mount config") ||
		strings.Contains(msg, "mount path")
}

// isFileSharingError reports whether a Docker API error is caused by a bind
// mount path that isn't shared with Docker Desktop's VM. This happens when
// symlinks resolve to paths outside the default shared directories (e.g.
// Nix Home Manager symlinks resolving to /nix/store/).
func isFileSharingError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "is not shared from the host") ||
		strings.Contains(msg, "is not known to Docker")
}

// fileSharingHint extracts the unshared path prefix from a Docker Desktop
// file sharing error and returns a user-friendly message. Returns empty
// string if the error doesn't match.
func fileSharingHint(err error) string {
	if !isFileSharingError(err) {
		return ""
	}
	msg := err.Error()

	// Extract the path from "The path /nix/store/... is not shared from the host".
	prefix := "The path "
	idx := strings.Index(msg, prefix)
	if idx < 0 {
		return "a mount path is not accessible to Docker Desktop — add the path to Docker Desktop → Settings → Resources → File Sharing"
	}
	path := msg[idx+len(prefix):]
	if spaceIdx := strings.Index(path, " "); spaceIdx > 0 {
		path = path[:spaceIdx]
	}

	// Suggest the top-level directory (e.g. /nix) rather than the full path.
	parts := strings.SplitN(strings.TrimPrefix(path, "/"), "/", 2)
	shareDir := "/" + parts[0]

	return fmt.Sprintf(
		"mount path %s is not accessible to Docker Desktop — add %q to Docker Desktop → Settings → Resources → File Sharing",
		path, shareDir,
	)
}

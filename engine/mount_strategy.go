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

// isMountError reports whether a Docker API error is related to a bind mount
// failure (e.g. source path doesn't exist or can't be stat'd). Used to narrow
// the socket mount retry to mount-specific failures.
func isMountError(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "mount source") ||
		strings.Contains(msg, "mount config") ||
		strings.Contains(msg, "mount path")
}

package engine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/docker/docker/api/types/mount"

	"github.com/thesimonho/warden/api"
)

// buildMounts partitions the given mounts into legacy bind strings (for
// regular file/directory mounts) and structured Docker mounts (for Unix
// domain sockets). Socket mounts use the structured mount.Mount API
// because Docker's legacy Binds format auto-creates missing host paths
// as directories, which fails for sockets on macOS with Docker Desktop.
//
// For local projects, the host directory is mounted at containerWorkspaceDir
// (typically /home/warden/<name>). For remote projects, projectPath is empty
// and the workspace mount is skipped.
func buildMounts(projectPath, containerWorkspaceDir string, mounts []api.Mount) (binds []string, structured []mount.Mount, err error) {
	if projectPath != "" {
		binds = append(binds, fmt.Sprintf("%s:%s", projectPath, containerWorkspaceDir))
	}

	for _, m := range mounts {
		if !filepath.IsAbs(m.HostPath) {
			return nil, nil, fmt.Errorf("mount host path must be absolute: %s", m.HostPath)
		}
		if !filepath.IsAbs(m.ContainerPath) {
			return nil, nil, fmt.Errorf("mount container path must be absolute: %s", m.ContainerPath)
		}

		if m.IsSocket {
			structured = append(structured, mount.Mount{
				Type:     mount.TypeBind,
				Source:   m.HostPath,
				Target:   m.ContainerPath,
				ReadOnly: m.ReadOnly,
			})
			continue
		}

		bind := fmt.Sprintf("%s:%s", m.HostPath, m.ContainerPath)
		if m.ReadOnly {
			bind += ":ro"
		}
		binds = append(binds, bind)
	}

	return binds, structured, nil
}

// isSocketPath reports whether the given host path is a Unix domain socket.
// Used when reconstructing mount metadata from Docker inspect data.
func isSocketPath(path string) bool {
	fi, err := os.Stat(path)
	if err != nil {
		return false
	}
	return fi.Mode().Type()&os.ModeSocket != 0
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

package engine

import (
	"fmt"
	"path/filepath"

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

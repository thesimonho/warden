package engine

import (
	"fmt"
	"path/filepath"
)

// buildBindMounts constructs the bind mount strings for container creation.
// The project directory is mounted at containerWorkspaceDir (typically
// /home/dev/<name>). Additional mounts are appended with optional :ro suffix.
func buildBindMounts(projectPath, containerWorkspaceDir string, mounts []Mount) ([]string, error) {
	binds := []string{fmt.Sprintf("%s:%s", projectPath, containerWorkspaceDir)}

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

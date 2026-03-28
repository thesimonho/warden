//go:build linux

package runtime

import (
	"os"
	"path/filepath"
)

// socketCandidates returns the ordered list of socket paths to try for a
// runtime on Linux.
func socketCandidates(rt Runtime) []string {
	switch rt {
	case RuntimeDocker:
		var candidates []string
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			candidates = append(candidates, host)
		}
		candidates = append(candidates, "/var/run/docker.sock")
		return candidates

	case RuntimePodman:
		var candidates []string
		if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
			candidates = append(candidates, filepath.Join(xdg, "podman", "podman.sock"))
		}
		candidates = append(candidates, "/run/podman/podman.sock")
		return candidates

	default:
		return nil
	}
}

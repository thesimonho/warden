//go:build darwin

package runtime

import (
	"os"
	"path/filepath"
)

// socketCandidates returns the ordered list of socket paths to try for a
// runtime on macOS. Covers Docker Desktop, Colima, OrbStack, and Podman
// machine — the common Docker-compatible runtimes on macOS.
func socketCandidates(rt Runtime) []string {
	home, _ := os.UserHomeDir()

	switch rt {
	case RuntimeDocker:
		var candidates []string
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			candidates = append(candidates, host)
		}
		if home != "" {
			candidates = append(candidates,
				filepath.Join(home, ".docker", "run", "docker.sock"),
				filepath.Join(home, ".colima", "default", "docker.sock"),
				filepath.Join(home, ".orbstack", "run", "docker.sock"),
			)
		}
		candidates = append(candidates, "/var/run/docker.sock")
		return candidates

	case RuntimePodman:
		var candidates []string
		if home != "" {
			candidates = append(candidates,
				filepath.Join(home, ".local", "share", "containers", "podman", "machine", "podman.sock"),
			)
		}
		candidates = append(candidates, "/var/run/podman/podman.sock")
		return candidates

	default:
		return nil
	}
}

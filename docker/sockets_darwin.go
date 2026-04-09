//go:build darwin

package docker

import (
	"os"
	"path/filepath"
)

// socketCandidates returns the ordered list of Docker socket paths to try on
// macOS. Checks DOCKER_HOST, then the active Docker context endpoint, then
// well-known socket paths for Docker Desktop, Colima, and OrbStack.
func socketCandidates() []string {
	home, _ := os.UserHomeDir()

	var candidates []string
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		candidates = append(candidates, host)
	}
	if ctx := contextSocketPath(); ctx != "" {
		candidates = append(candidates, ctx)
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
}

//go:build linux

package docker

import "os"

// socketCandidates returns the ordered list of Docker socket paths to try on Linux.
// Checks DOCKER_HOST first, then the active Docker context endpoint, then the
// standard socket path.
func socketCandidates() []string {
	var candidates []string
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		candidates = append(candidates, host)
	}
	if ctx := contextSocketPath(); ctx != "" {
		candidates = append(candidates, ctx)
	}
	candidates = append(candidates, "/var/run/docker.sock")
	return candidates
}

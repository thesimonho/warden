//go:build linux

package docker

import "os"

// socketCandidates returns the ordered list of Docker socket paths to try on Linux.
func socketCandidates() []string {
	var candidates []string
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		candidates = append(candidates, host)
	}
	candidates = append(candidates, "/var/run/docker.sock")
	return candidates
}

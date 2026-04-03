//go:build linux

package runtime

import "os"

// socketCandidates returns the ordered list of Docker socket paths to try on Linux.
func socketCandidates(rt Runtime) []string {
	switch rt {
	case RuntimeDocker:
		var candidates []string
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			candidates = append(candidates, host)
		}
		candidates = append(candidates, "/var/run/docker.sock")
		return candidates

	default:
		return nil
	}
}

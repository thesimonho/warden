//go:build windows

package docker

import "os"

// socketCandidates returns the ordered list of Docker named pipe paths to try
// on Windows. Checks DOCKER_HOST, then the active Docker context endpoint,
// then well-known named pipes.
func socketCandidates() []string {
	var candidates []string
	if host := os.Getenv("DOCKER_HOST"); host != "" {
		candidates = append(candidates, host)
	}
	if ctx := contextSocketPath(); ctx != "" {
		candidates = append(candidates, ctx)
	}
	candidates = append(candidates,
		`//./pipe/docker_engine`,
		`//./pipe/dockerDesktopLinuxEngine`,
	)
	return candidates
}

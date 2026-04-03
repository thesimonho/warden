//go:build windows

package runtime

import "os"

// socketCandidates returns the ordered list of Docker named pipe paths to try
// on Windows.
func socketCandidates(rt Runtime) []string {
	switch rt {
	case RuntimeDocker:
		var candidates []string
		if host := os.Getenv("DOCKER_HOST"); host != "" {
			candidates = append(candidates, host)
		}
		candidates = append(candidates,
			`//./pipe/docker_engine`,
			`//./pipe/dockerDesktopLinuxEngine`,
		)
		return candidates

	default:
		return nil
	}
}

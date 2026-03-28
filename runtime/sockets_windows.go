//go:build windows

package runtime

import "os"

// socketCandidates returns the ordered list of socket/pipe paths to try for a
// runtime on Windows. Docker Desktop and Podman machine both expose named pipes.
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

	case RuntimePodman:
		var candidates []string
		candidates = append(candidates,
			`//./pipe/podman-machine-default`,
		)
		return candidates

	default:
		return nil
	}
}

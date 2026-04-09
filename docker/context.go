package docker

import (
	"context"
	"os/exec"
	"strings"
	"time"
)

// contextSocketPath returns the Docker socket path from the active Docker
// context, or empty string if detection fails. Uses the Docker CLI to inspect
// the current context endpoint, which respects `docker context use` and the
// DOCKER_CONTEXT environment variable.
func contextSocketPath() string {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "context", "inspect",
		"--format", "{{.Endpoints.docker.Host}}",
	).Output()
	if err != nil {
		return ""
	}

	host := strings.TrimSpace(string(out))
	if host == "" {
		return ""
	}

	// Strip the scheme prefix to return a raw socket path. SocketHost()
	// will re-add it when creating the Docker client.
	host = strings.TrimPrefix(host, "unix://")
	host = strings.TrimPrefix(host, "npipe://")

	return host
}

// Package runtime detects the Docker container runtime
// and resolves its API socket path.
package runtime

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
)

// Runtime identifies a container runtime engine.
type Runtime string

const (
	// RuntimeDocker is the Docker container runtime.
	RuntimeDocker Runtime = "docker"
)

// RuntimeInfo describes a detected container runtime.
type RuntimeInfo struct {
	// Name is the runtime identifier ("docker").
	Name Runtime `json:"name"`
	// Available indicates whether the runtime's API socket is reachable.
	Available bool `json:"available"`
	// SocketPath is the filesystem path to the runtime's API socket.
	SocketPath string `json:"socketPath"`
	// Version is the runtime's reported API version, if available.
	Version string `json:"version,omitempty"`
}

// socketCandidates is defined per-platform in sockets_{linux,darwin,windows}.go.

// SocketHost converts a raw socket path to a Docker client host URI.
//
// Handles three forms:
//   - Absolute Unix paths (/var/run/docker.sock) → unix:///var/run/docker.sock
//   - Windows named pipes (//./pipe/docker_engine) → npipe:////./pipe/docker_engine
//   - URIs that already have a scheme (unix://, npipe://, tcp://) → pass through
func SocketHost(socketPath string) string {
	if strings.Contains(socketPath, "://") {
		return socketPath
	}
	if strings.HasPrefix(socketPath, "//./pipe/") {
		return "npipe://" + socketPath
	}
	return "unix://" + socketPath
}

// probeSocket attempts to connect to the runtime API at the given socket path
// and returns version info if successful.
func probeSocket(ctx context.Context, socketPath string) (string, error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(SocketHost(socketPath)),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return "", fmt.Errorf("creating client for %s: %w", socketPath, err)
	}
	defer cli.Close() //nolint:errcheck

	ping, err := cli.Ping(ctx)
	if err != nil {
		return "", fmt.Errorf("pinging %s: %w", socketPath, err)
	}

	return ping.APIVersion, nil
}

// probeBinary checks whether the runtime binary is installed and returns its
// version. This serves as a fallback when socket probing fails.
func probeBinary(ctx context.Context, rt Runtime) (string, error) {
	binName := string(rt)
	binPath, err := exec.LookPath(binName)
	if err != nil {
		return "", fmt.Errorf("%s binary not found: %w", binName, err)
	}

	out, err := exec.CommandContext(ctx, binPath, "version", "--format", "{{.Client.Version}}").Output()
	if err != nil {
		return "", fmt.Errorf("getting %s version: %w", binName, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// DetectAvailable checks whether Docker is available and returns its status.
// Docker is probed first by connecting to its API socket, then by checking
// for the binary as a fallback.
func DetectAvailable(ctx context.Context) []RuntimeInfo {
	info := RuntimeInfo{Name: RuntimeDocker}

	for _, socketPath := range socketCandidates(RuntimeDocker) {
		version, err := probeSocket(ctx, socketPath)
		if err == nil {
			info.Available = true
			info.SocketPath = socketPath
			info.Version = version
			break
		}
	}

	// Fall back to binary detection if socket probe failed.
	if !info.Available {
		if version, err := probeBinary(ctx, RuntimeDocker); err == nil {
			info.Available = true
			info.Version = version
		}
	}

	return []RuntimeInfo{info}
}

// SocketForRuntime returns the first reachable Docker socket path,
// or an empty string if no socket is found (allowing fallback to client.FromEnv).
func SocketForRuntime(ctx context.Context) string {
	for _, socketPath := range socketCandidates(RuntimeDocker) {
		_, err := probeSocket(ctx, socketPath)
		if err == nil {
			return socketPath
		}
	}
	return ""
}

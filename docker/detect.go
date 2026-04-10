// Package docker detects the Docker daemon and resolves its API socket path.
package docker

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/docker/docker/client"
)

// Name is the identifier for the Docker container runtime.
const Name = "docker"

// Info describes the detected Docker runtime status.
type Info struct {
	// Name is the runtime identifier ("docker").
	Name string `json:"name"`
	// Available indicates whether the Docker API socket is reachable.
	Available bool `json:"available"`
	// IsDesktop indicates whether the Docker runtime is Docker Desktop
	// (as opposed to native Docker Engine, Colima, OrbStack, etc.).
	// Detected via the OperatingSystem field from the Docker API info
	// endpoint, which returns "Docker Desktop" for all Docker Desktop
	// installations regardless of host OS.
	IsDesktop bool `json:"isDesktop"`
	// SocketPath is the filesystem path to the Docker API socket.
	SocketPath string `json:"socketPath"`
	// Version is Docker's reported API version, if available.
	Version string `json:"version,omitempty"`
}

// socketCandidates returns platform-specific Docker socket paths (build-tagged).

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

// probeSocket attempts to connect to the Docker API at the given socket path
// and returns the API version and whether the runtime is Docker Desktop.
func probeSocket(ctx context.Context, socketPath string) (version string, isDesktop bool, err error) {
	cli, err := client.NewClientWithOpts(
		client.WithHost(SocketHost(socketPath)),
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return "", false, fmt.Errorf("creating client for %s: %w", socketPath, err)
	}
	defer cli.Close() //nolint:errcheck

	ping, pingErr := cli.Ping(ctx)
	if pingErr != nil {
		return "", false, fmt.Errorf("pinging %s: %w", socketPath, pingErr)
	}

	// Check OperatingSystem to detect Docker Desktop. This is reliable
	// across all platforms — Docker Desktop always reports "Docker Desktop"
	// regardless of host OS.
	info, infoErr := cli.Info(ctx)
	if infoErr == nil {
		isDesktop = strings.HasPrefix(info.OperatingSystem, "Docker Desktop")
	}

	return ping.APIVersion, isDesktop, nil
}

// probeBinary checks whether the docker binary is installed and returns its
// version. This serves as a fallback when socket probing fails.
func probeBinary(ctx context.Context) (string, error) {
	binPath, err := exec.LookPath(Name)
	if err != nil {
		return "", fmt.Errorf("%s binary not found: %w", Name, err)
	}

	out, err := exec.CommandContext(ctx, binPath, "version", "--format", "{{.Client.Version}}").Output()
	if err != nil {
		return "", fmt.Errorf("getting %s version: %w", Name, err)
	}

	return strings.TrimSpace(string(out)), nil
}

// Detect checks whether Docker is available and returns its status.
// Docker is probed first by connecting to its API socket, then by checking
// for the binary as a fallback.
func Detect(ctx context.Context) Info {
	info := Info{Name: Name}

	for _, socketPath := range socketCandidates() {
		ver, desktop, err := probeSocket(ctx, socketPath)
		if err == nil {
			info.Available = true
			info.SocketPath = socketPath
			info.Version = ver
			info.IsDesktop = desktop
			return info
		}
	}

	// Fall back to binary detection if socket probe failed.
	if version, err := probeBinary(ctx); err == nil {
		info.Available = true
		info.Version = version
	}

	return info
}


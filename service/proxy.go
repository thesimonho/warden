package service

import (
	"context"
	"fmt"
	"slices"
)

// ProxyPort validates that a port is declared for the given project and
// returns the host and target port to proxy to. On native Docker the
// target is the container's bridge IP on the requested port. On Docker
// Desktop (where bridge IPs are unreachable from the host) the target
// is a local port bridge that tunnels via docker exec.
func (s *Service) ProxyPort(ctx context.Context, projectID, agentType string, port int) (string, int, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return "", 0, err
	}

	if !slices.Contains(parsePortList(project.ForwardedPorts), port) {
		return "", 0, fmt.Errorf("%w: port %d is not in forwarded ports", ErrInvalidInput, port)
	}

	// Docker Desktop: bridge IPs are inside the VM and unreachable from
	// the host. Use a local TCP bridge that tunnels via docker exec.
	if s.dockerInfo.IsDesktop {
		return s.proxyViaPortBridge(project.ContainerID, project.ContainerName, port)
	}

	// Native Docker: bridge IP is directly reachable from the host.
	ip, err := s.docker.ContainerIP(ctx, project.ContainerID)
	if err != nil {
		return "", 0, fmt.Errorf("resolving container IP: %w", err)
	}
	return ip, port, nil
}

// proxyViaPortBridge returns the host and port of a local TCP bridge
// that tunnels connections into the container via docker exec.
func (s *Service) proxyViaPortBridge(containerID, containerName string, port int) (string, int, error) {
	bridge, err := s.getOrStartPortBridge(containerID, containerName, port)
	if err != nil {
		return "", 0, fmt.Errorf("starting port bridge: %w", err)
	}
	return "127.0.0.1", bridge.ListenPort(), nil
}

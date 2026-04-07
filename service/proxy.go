package service

import (
	"context"
	"fmt"
	"slices"
)

// ProxyPort validates that a port is declared for the given project and
// returns the container's bridge network IP address. Returns ErrNotFound
// if the project doesn't exist, ErrInvalidInput if the port is not in
// the declared forwarded ports list.
func (s *Service) ProxyPort(ctx context.Context, projectID, agentType string, port int) (string, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return "", err
	}

	if !slices.Contains(parsePortList(project.ForwardedPorts), port) {
		return "", fmt.Errorf("%w: port %d is not in forwarded ports", ErrInvalidInput, port)
	}

	ip, err := s.docker.ContainerIP(ctx, project.ContainerID)
	if err != nil {
		return "", fmt.Errorf("resolving container IP: %w", err)
	}
	return ip, nil
}

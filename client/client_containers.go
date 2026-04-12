package client

import (
	"context"

	"github.com/thesimonho/warden/api"
)

// CheckContainerName reports whether a container name is available for use.
func (c *Client) CheckContainerName(ctx context.Context, name string) (*api.CheckNameResult, error) {
	var resp api.CheckNameResult
	if err := c.post(ctx, "/api/v1/containers/check-name", api.CheckNameRequest{Name: name}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CreateContainer creates a new container for the given project.
func (c *Client) CreateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	if err := c.post(ctx, projectPath(projectID, agentType)+"/container", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetProjectCosts removes all cost history for a project.
func (c *Client) ResetProjectCosts(ctx context.Context, projectID, agentType string) error {
	return c.delete(ctx, projectPath(projectID, agentType)+"/costs")
}

// PurgeProjectAudit removes all audit events for a project.
func (c *Client) PurgeProjectAudit(ctx context.Context, projectID, agentType string) error {
	return c.delete(ctx, projectPath(projectID, agentType)+"/audit")
}

// DeleteContainer stops and removes the container for the given project.
func (c *Client) DeleteContainer(ctx context.Context, projectID, agentType string) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	path := projectPath(projectID, agentType) + "/container"
	if err := c.deleteWithBody(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// InspectContainer returns the configuration of the project's container.
func (c *Client) InspectContainer(ctx context.Context, projectID, agentType string) (*api.ContainerConfig, error) {
	var cfg api.ContainerConfig
	path := projectPath(projectID, agentType) + "/container/config"
	if err := c.get(ctx, path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// UpdateContainer recreates the project's container with updated configuration.
func (c *Client) UpdateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*api.ContainerResult, error) {
	var resp api.ContainerResult
	path := projectPath(projectID, agentType) + "/container"
	if err := c.put(ctx, path, req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ValidateContainer checks whether the project's container has Warden infrastructure.
func (c *Client) ValidateContainer(ctx context.Context, projectID, agentType string) (*api.ValidateContainerResult, error) {
	var resp api.ValidateContainerResult
	path := projectPath(projectID, agentType) + "/container/validate"
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

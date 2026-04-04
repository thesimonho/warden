package client

import (
	"context"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
)

// ListProjects returns all configured projects with container state, cost,
// and attention data. Each [engine.Project] includes State ("running",
// "exited", "not-found"), NeedsInput (true when Claude needs attention),
// NotificationType ("permission_prompt", "idle_prompt", "elicitation_dialog"),
// ActiveWorktreeCount, TotalCost (USD), and NetworkMode.
func (c *Client) ListProjects(ctx context.Context) ([]engine.Project, error) {
	var projects []engine.Project
	if err := c.get(ctx, "/api/v1/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// AddProject registers a project directory in Warden.
func (c *Client) AddProject(ctx context.Context, name, hostPath, agentType string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	body := map[string]string{"name": name, "projectPath": hostPath, "agentType": agentType}
	if err := c.post(ctx, "/api/v1/projects", body, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveProject removes a project from the database by project ID.
func (c *Client) RemoveProject(ctx context.Context, projectID, agentType string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	path := projectPath(projectID, agentType)
	if err := c.deleteWithBody(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// StopProject stops the container with the given ID.
func (c *Client) StopProject(ctx context.Context, id, agentType string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	path := projectPath(id, agentType) + "/stop"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RestartProject restarts the container with the given ID.
func (c *Client) RestartProject(ctx context.Context, id, agentType string) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	path := projectPath(id, agentType) + "/restart"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

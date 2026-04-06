package client

import (
	"context"

	"github.com/thesimonho/warden/api"
)

// ListProjects returns all configured projects with container state, cost,
// and attention data. Each [api.ProjectResponse] includes State ("running",
// "exited", "not-found"), NeedsInput (true when Claude needs attention),
// NotificationType ("permission_prompt", "idle_prompt", "elicitation_dialog"),
// ActiveWorktreeCount, TotalCost (USD), and NetworkMode.
func (c *Client) ListProjects(ctx context.Context) ([]api.ProjectResponse, error) {
	var projects []api.ProjectResponse
	if err := c.get(ctx, "/api/v1/projects", &projects); err != nil {
		return nil, err
	}
	return projects, nil
}

// GetProject returns a single project enriched with container state, cost, and attention data.
func (c *Client) GetProject(ctx context.Context, projectID, agentType string) (*api.ProjectResponse, error) {
	var resp api.ProjectResponse
	path := projectPath(projectID, agentType)
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetProjectCosts returns session-level cost data for a project.
func (c *Client) GetProjectCosts(ctx context.Context, projectID, agentType string) (*api.ProjectCostsResponse, error) {
	var resp api.ProjectCostsResponse
	path := projectPath(projectID, agentType) + "/costs"
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// GetBudgetStatus returns the budget state for a project.
func (c *Client) GetBudgetStatus(ctx context.Context, projectID, agentType string) (*api.BudgetStatusResponse, error) {
	var resp api.BudgetStatusResponse
	path := projectPath(projectID, agentType) + "/budget"
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// AddProject registers a project directory in Warden.
func (c *Client) AddProject(ctx context.Context, req api.AddProjectRequest) (*api.ProjectResult, error) {
	var resp api.ProjectResult
	if err := c.post(ctx, "/api/v1/projects", req, &resp); err != nil {
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

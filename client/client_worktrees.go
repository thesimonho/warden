package client

import (
	"context"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
)

// --- Worktrees ---

// ListWorktrees returns all worktrees for the given container.
func (c *Client) ListWorktrees(ctx context.Context, projectID, agentType string) ([]engine.Worktree, error) {
	var worktrees []engine.Worktree
	path := projectPath(projectID, agentType) + "/worktrees"
	if err := c.get(ctx, path, &worktrees); err != nil {
		return nil, err
	}
	return worktrees, nil
}

// CreateWorktree creates a new git worktree and connects a terminal.
func (c *Client) CreateWorktree(ctx context.Context, projectID, agentType, name string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees"
	if err := c.post(ctx, path, map[string]string{"name": name}, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ConnectTerminal starts or reconnects the terminal process (tmux session)
// for a worktree. Must be called before [AttachTerminal] to ensure the
// process exists. The terminal process continues running in the background
// even if no viewer is attached, allowing Claude Code to work independently.
//
// If the terminal is already running, this is a no-op that returns success.
func (c *Client) ConnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID + "/connect"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DisconnectTerminal closes the terminal viewer for a worktree.
func (c *Client) DisconnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID + "/disconnect"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// KillWorktreeProcess kills the terminal process for a worktree.
func (c *Client) KillWorktreeProcess(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID + "/kill"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// ResetWorktree clears all history for a worktree without removing it.
func (c *Client) ResetWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID + "/reset"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// RemoveWorktree fully removes a worktree.
func (c *Client) RemoveWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*api.WorktreeResult, error) {
	var resp api.WorktreeResult
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID
	if err := c.deleteWithBody(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// CleanupWorktrees removes orphaned worktree directories.
func (c *Client) CleanupWorktrees(ctx context.Context, projectID, agentType string) ([]string, error) {
	var resp struct {
		Removed []string `json:"removed"`
	}
	path := projectPath(projectID, agentType) + "/worktrees/cleanup"
	if err := c.post(ctx, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.Removed, nil
}

// GetWorktreeDiff returns uncommitted changes for a worktree.
func (c *Client) GetWorktreeDiff(ctx context.Context, projectID, agentType, worktreeID string) (*api.DiffResponse, error) {
	var resp api.DiffResponse
	path := projectPath(projectID, agentType) + "/worktrees/" + worktreeID + "/diff"
	if err := c.get(ctx, path, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

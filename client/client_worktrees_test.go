package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/service"
)

func TestConnectTerminal(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x/connect",
		http.StatusCreated, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	resp, err := c.ConnectTerminal(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "feature-x" {
		t.Errorf("expected worktree ID 'feature-x', got %q", resp.WorktreeID)
	}
}

func TestDisconnectTerminal(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x/disconnect",
		http.StatusOK, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	_, err := c.DisconnectTerminal(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKillWorktreeProcess(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x/kill",
		http.StatusOK, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	_, err := c.KillWorktreeProcess(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetWorktree(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x/reset",
		http.StatusOK, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	_, err := c.ResetWorktree(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveWorktree(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x",
		http.StatusOK, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	_, err := c.RemoveWorktree(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestCleanupWorktrees(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees/cleanup",
		http.StatusOK, map[string][]string{"removed": {"orphan-1", "orphan-2"}})

	removed, err := c.CleanupWorktrees(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
}

func TestGetWorktreeDiff(t *testing.T) {
	t.Parallel()

	diff := api.DiffResponse{
		RawDiff: "diff --git a/main.go b/main.go\n",
		Files: []api.DiffFileSummary{
			{Path: "main.go", Status: "modified", Additions: 5, Deletions: 2},
		},
	}
	c := newTestServer(t, "GET", "/api/v1/projects/abc123def456/claude-code/worktrees/feature-x/diff",
		http.StatusOK, diff)

	resp, err := c.GetWorktreeDiff(context.Background(), "abc123def456", "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(resp.Files))
	}
	if resp.Files[0].Additions != 5 {
		t.Errorf("expected 5 additions, got %d", resp.Files[0].Additions)
	}
}

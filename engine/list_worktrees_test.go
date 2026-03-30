package engine

import (
	"context"
	"strings"
	"testing"

	"github.com/thesimonho/warden/agent"
)

// newTestClient creates an EngineClient backed by the exec mock API.
func newTestClient(mockAPI *execMockAPI) *EngineClient {
	return &EngineClient{
		api:           mockAPI,
		agentRegistry: agent.NewRegistry(),
		runtimeName:   "podman",
	}
}

func TestListWorktrees_GitRepo_NoSuCommand(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", `worktree /project
HEAD abc123
branch refs/heads/main

worktree /project/.worktrees/feature-x
HEAD def456
branch refs/heads/feature-x
`)
	// ls for mergeTerminalWorktrees
	mock.onCmd("ls", "")
	// enrichWorktreeState batch command
	mock.onCmd("echo", "")
	// pgrep for abduco check (returns 0 = not alive)
	mock.onCmd("pgrep", "0\n")

	dc := newTestClient(mock)

	worktrees, err := dc.listWorktreesWithHint(context.Background(), "ctr-123", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	if worktrees[0].ID != "main" {
		t.Errorf("expected first worktree ID 'main', got %q", worktrees[0].ID)
	}
	if worktrees[1].ID != "feature-x" {
		t.Errorf("expected second worktree ID 'feature-x', got %q", worktrees[1].ID)
	}

	// Verify no exec call uses "su" — this was the root cause of the dead terminal bug.
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == "su" {
			t.Errorf("exec call uses 'su' command which fails in rootless containers: %v", call.Cmd)
		}
	}
}

func TestListWorktrees_NonGitRepo_ReturnsSingleWorktree(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("ls", "")
	mock.onCmd("echo", "")

	dc := newTestClient(mock)

	worktrees, err := dc.listWorktreesWithHint(context.Background(), "ctr-456", false, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}

	if worktrees[0].ID != "main" {
		t.Errorf("expected worktree ID 'main', got %q", worktrees[0].ID)
	}
	if worktrees[0].Path != "/project" {
		t.Errorf("expected path '/project', got %q", worktrees[0].Path)
	}
}

func TestListWorktrees_GitDiscovery_UsesGitDirectly(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", `worktree /project
HEAD abc123
branch refs/heads/main
`)
	mock.onCmd("ls", "")
	mock.onCmd("echo", "")

	dc := newTestClient(mock)

	_, err := dc.listWorktreesWithHint(context.Background(), "ctr-789", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	// Find the git worktree list call and verify its arguments.
	var gitCall *execCall
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == "git" {
			gitCall = &call
			break
		}
	}

	if gitCall == nil {
		t.Fatal("expected a 'git' exec call for worktree discovery")
		return // unreachable but helps staticcheck
	}

	// Should be: git -C /project -c safe.directory=/project worktree list --porcelain
	expectedArgs := []string{"git", "-C", "/project", "-c", "safe.directory=/project", "worktree", "list", "--porcelain"}
	if len(gitCall.Cmd) != len(expectedArgs) {
		t.Fatalf("expected git command %v, got %v", expectedArgs, gitCall.Cmd)
	}
	for i, arg := range expectedArgs {
		if gitCall.Cmd[i] != arg {
			t.Errorf("git command arg[%d]: expected %q, got %q", i, arg, gitCall.Cmd[i])
		}
	}
}

func TestListWorktrees_MergesTerminalWorktrees(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Git reports only main worktree.
	mock.onCmd("git", `worktree /project
HEAD abc123
branch refs/heads/main
`)
	// Terminal directory has an extra worktree being created.
	mock.onCmd("ls", "pending-feature\n")
	mock.onCmd("echo", "")

	dc := newTestClient(mock)

	worktrees, err := dc.listWorktreesWithHint(context.Background(), "ctr-merge", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees (main + pending), got %d", len(worktrees))
	}

	if worktrees[1].ID != "pending-feature" {
		t.Errorf("expected merged worktree 'pending-feature', got %q", worktrees[1].ID)
	}
}

func TestEnrichWorktreeState_PgrepUsesCharClassTrick(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("echo", "")

	dc := newTestClient(mock)

	worktrees := []Worktree{
		{ID: "feature-x", ProjectID: "ctr-pgrep", State: WorktreeStateDisconnected},
	}

	dc.enrichWorktreeState(context.Background(), "ctr-pgrep", worktrees)

	// Verify the batch command uses [a]bduco, not abduco.
	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 3 && call.Cmd[0] == "sh" && call.Cmd[1] == "-c" {
			shellCmd := call.Cmd[2]
			if strings.Contains(shellCmd, "abduco") && !strings.Contains(shellCmd, "[a]bduco") {
				t.Errorf("pgrep command uses bare 'abduco' instead of '[a]bduco' which causes self-matching: %s", shellCmd)
			}
		}
	}
}

func TestIsAbducoSessionAlive_PgrepUsesCharClassTrick(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("pgrep", "0\n")

	dc := newTestClient(mock)

	dc.isAbducoSessionAlive(context.Background(), "ctr-abduco", "test-wt")

	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 3 && call.Cmd[0] == "sh" && call.Cmd[1] == "-c" {
			shellCmd := call.Cmd[2]
			if strings.Contains(shellCmd, `"abduco`) && !strings.Contains(shellCmd, `"[a]bduco`) {
				t.Errorf("pgrep command uses bare 'abduco' instead of '[a]bduco': %s", shellCmd)
			}
		}
	}
}

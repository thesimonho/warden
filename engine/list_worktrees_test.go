package engine

import (
	"context"
	"testing"

	"github.com/thesimonho/warden/agent"
)

// newTestClient creates an EngineClient backed by the exec mock API.
func newTestClient(mockAPI *execMockAPI) *EngineClient {
	return &EngineClient{
		api:           mockAPI,
		agentRegistry: agent.NewRegistry(),
	}
}

func TestListWorktrees_GitRepo_DiscoversWorktreesWithBranches(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("---GIT_END---", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /project/.claude/worktrees/feature-x\nHEAD def456\nbranch refs/heads/feature-x\n---GIT_END---\n---LS_END---\n")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-123", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify identity and metadata of discovered worktrees.
	if worktrees[0].ID != "main" {
		t.Errorf("first worktree ID: got %q, want %q", worktrees[0].ID, "main")
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("first worktree branch: got %q, want %q", worktrees[0].Branch, "main")
	}
	if worktrees[0].Path != "/project" {
		t.Errorf("first worktree path: got %q, want %q", worktrees[0].Path, "/project")
	}

	if worktrees[1].ID != "feature-x" {
		t.Errorf("second worktree ID: got %q, want %q", worktrees[1].ID, "feature-x")
	}
	if worktrees[1].Branch != "feature-x" {
		t.Errorf("second worktree branch: got %q, want %q", worktrees[1].Branch, "feature-x")
	}
	if worktrees[1].Path != "/project/.claude/worktrees/feature-x" {
		t.Errorf("second worktree path: got %q, want %q", worktrees[1].Path, "/project/.claude/worktrees/feature-x")
	}

	// Verify no exec call uses "su" — regression guard for dead terminal bug
	// in rootless containers.
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == "su" {
			t.Errorf("exec call uses 'su' command which fails in rootless containers: %v", call.Cmd)
		}
	}
}

func TestListWorktrees_GitRepo_SkipsPrunableWorktrees(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Git reports main + a prunable worktree that should be filtered out.
	mock.onCmd("---GIT_END---", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /project/.claude/worktrees/stale-branch\nHEAD 000000\nbranch refs/heads/stale-branch\nprunable gitdir file points to non-existent location\n---GIT_END---\n---LS_END---\n")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-prune", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree (prunable filtered), got %d", len(worktrees))
	}
	if worktrees[0].ID != "main" {
		t.Errorf("expected only 'main' worktree, got %q", worktrees[0].ID)
	}
}

func TestListWorktrees_NonGitRepo_ReturnsSingleWorktree(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("ls", "")
	mock.onCmd("echo", "")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-456", false, false)
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

func TestListWorktrees_MergesTerminalWorktrees(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// The combined script runs git + ls in a single sh -c command.
	// Register a response matching the unique GIT_END delimiter so
	// it doesn't collide with other registered commands.
	mock.onCmd("---GIT_END---", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n---GIT_END---\npending-feature\n---LS_END---\n")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-merge", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees (main + pending), got %d", len(worktrees))
	}

	if worktrees[1].ID != "pending-feature" {
		t.Errorf("expected merged worktree 'pending-feature', got %q", worktrees[1].ID)
	}
	// Merged worktrees default to stopped state.
	if worktrees[1].State != WorktreeStateStopped {
		t.Errorf("merged worktree state: got %q, want %q", worktrees[1].State, WorktreeStateStopped)
	}
}

func TestListWorktrees_MergeSkipsDuplicates(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Git and terminal dir both report the same worktree — should not duplicate.
	mock.onCmd("---GIT_END---", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /project/.claude/worktrees/feature-x\nHEAD def456\nbranch refs/heads/feature-x\n---GIT_END---\nfeature-x\n---LS_END---\n")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-dedup", true, false)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees (no duplicate), got %d", len(worktrees))
	}
}

func TestEnrichWorktreeState_SetsConnectedWhenSessionAlive(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batch enrichment output: session alive (1), no exit code.
	mock.onCmd("TMUX", `---WT_START:feature-x---
---EXIT_END---
1
---SESSION_END---
node
---PANE_END---`)

	ec := newTestClient(mock)

	worktrees := []Worktree{
		{ID: "feature-x", ProjectID: "ctr-enrich", State: WorktreeStateStopped},
	}

	ec.enrichWorktreeState(context.Background(), "ctr-enrich", worktrees)

	if worktrees[0].State != WorktreeStateConnected {
		t.Errorf("worktree state: got %q, want %q", worktrees[0].State, WorktreeStateConnected)
	}
	if worktrees[0].ExitCode != nil {
		t.Errorf("exit code should be nil for running agent, got %v", *worktrees[0].ExitCode)
	}
}

func TestEnrichWorktreeState_SetsShellWhenAgentExited(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batch enrichment output: session alive (1), exit code 0 (agent exited normally).
	mock.onCmd("TMUX", `---WT_START:my-wt---
0
---EXIT_END---
1
---SESSION_END---
bash
---PANE_END---`)

	ec := newTestClient(mock)

	worktrees := []Worktree{
		{ID: "my-wt", ProjectID: "ctr-shell", State: WorktreeStateStopped},
	}

	ec.enrichWorktreeState(context.Background(), "ctr-shell", worktrees)

	if worktrees[0].State != WorktreeStateShell {
		t.Errorf("worktree state: got %q, want %q", worktrees[0].State, WorktreeStateShell)
	}
	if worktrees[0].ExitCode == nil || *worktrees[0].ExitCode != 0 {
		t.Errorf("exit code: got %v, want 0", worktrees[0].ExitCode)
	}
}

func TestEnrichWorktreeState_StaysStoppedWhenSessionDead(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batch enrichment output: session dead (0), no exit code.
	mock.onCmd("TMUX", `---WT_START:dead-wt---
---EXIT_END---
0
---SESSION_END---

---PANE_END---`)

	ec := newTestClient(mock)

	worktrees := []Worktree{
		{ID: "dead-wt", ProjectID: "ctr-dead", State: WorktreeStateStopped},
	}

	ec.enrichWorktreeState(context.Background(), "ctr-dead", worktrees)

	if worktrees[0].State != WorktreeStateStopped {
		t.Errorf("worktree state: got %q, want %q", worktrees[0].State, WorktreeStateStopped)
	}
}

func TestEnrichWorktreeState_MultipleWorktrees(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Two worktrees: one connected, one exited with code 137 (killed).
	mock.onCmd("TMUX", `---WT_START:active---
---EXIT_END---
1
---SESSION_END---
node
---PANE_END---
---WT_START:killed---
137
---EXIT_END---
1
---SESSION_END---
bash
---PANE_END---`)

	ec := newTestClient(mock)

	worktrees := []Worktree{
		{ID: "active", ProjectID: "ctr-multi", State: WorktreeStateStopped},
		{ID: "killed", ProjectID: "ctr-multi", State: WorktreeStateStopped},
	}

	ec.enrichWorktreeState(context.Background(), "ctr-multi", worktrees)

	if worktrees[0].State != WorktreeStateConnected {
		t.Errorf("active worktree state: got %q, want %q", worktrees[0].State, WorktreeStateConnected)
	}
	if worktrees[1].State != WorktreeStateShell {
		t.Errorf("killed worktree state: got %q, want %q", worktrees[1].State, WorktreeStateShell)
	}
	if worktrees[1].ExitCode == nil || *worktrees[1].ExitCode != 137 {
		t.Errorf("killed worktree exit code: got %v, want 137", worktrees[1].ExitCode)
	}
}

// Regression: after the user Ctrl-C's the agent and drops to the fallback
// bash shell, the wrapper writes an exit_code file. If the user then manually
// runs the agent again from that shell, the wrapper never runs, so exit_code
// stays on disk. The pane's foreground process is `node` (the agent) again —
// the worktree should report connected, not stay stuck in shell state.
func TestEnrichWorktreeState_ManualRestartClearsShellState(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Stale exit_code 0 from the prior agent run, but pane foreground is now `node`.
	mock.onCmd("TMUX", `---WT_START:feature-x---
0
---EXIT_END---
1
---SESSION_END---
node
---PANE_END---`)

	ec := newTestClient(mock)

	// Start with a non-nil ExitCode from a prior poll cycle to verify that
	// the manual-restart branch clears it — otherwise the UI would show a
	// connected worktree with a stale exit code.
	prior := 0
	worktrees := []Worktree{
		{ID: "feature-x", ProjectID: "ctr-restart", State: WorktreeStateShell, ExitCode: &prior},
	}

	ec.enrichWorktreeState(context.Background(), "ctr-restart", worktrees)

	if worktrees[0].State != WorktreeStateConnected {
		t.Errorf("worktree state: got %q, want %q (agent running again via manual restart)", worktrees[0].State, WorktreeStateConnected)
	}
	if worktrees[0].ExitCode != nil {
		t.Errorf("exit code should be cleared when agent is running again, got %v", *worktrees[0].ExitCode)
	}
}

func TestIsSessionAlive_ReturnsTrue(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("tmux", "1\n")

	ec := newTestClient(mock)

	if !ec.isSessionAlive(context.Background(), "ctr-alive", "my-wt") {
		t.Error("expected isSessionAlive to return true when tmux reports session exists")
	}
}

func TestIsSessionAlive_ReturnsFalse(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("tmux", "0\n")

	ec := newTestClient(mock)

	if ec.isSessionAlive(context.Background(), "ctr-dead", "my-wt") {
		t.Error("expected isSessionAlive to return false when tmux reports no session")
	}
}

func TestListWorktrees_SkipEnrich_LeavesDefaultState(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n---GIT_END---\n---LS_END---\n")

	ec := newTestClient(mock)

	worktrees, err := ec.listWorktreesWithHint(context.Background(), "ctr-skip", true, true)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}

	if len(worktrees) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(worktrees))
	}
	// When skipEnrich is true, state stays at the default (stopped).
	if worktrees[0].State != WorktreeStateStopped {
		t.Errorf("worktree state with skipEnrich: got %q, want %q", worktrees[0].State, WorktreeStateStopped)
	}
}

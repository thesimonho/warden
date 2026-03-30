package engine

import (
	"context"
	"strings"
	"testing"
)

// primeGitRepoCache pre-warms the git repo cache so checkIsGitRepo
// doesn't fire an exec call that collides with other "git" mocks.
func primeGitRepoCache(ec *EngineClient, containerID string, isGit bool) {
	ec.gitRepoCache.Store(containerID, isGit)
}

// ---------------------------------------------------------------------------
// pruneGitWorktrees
// ---------------------------------------------------------------------------

func TestPruneGitWorktrees_CallsGitWorktreePrune(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", "")

	ec := newTestClient(mock)

	ec.pruneGitWorktrees(context.Background(), "ctr-prune")

	var found bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 3 && call.Cmd[0] == "git" {
			if call.Cmd[len(call.Cmd)-1] == "prune" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected 'git worktree prune' exec call")
	}
}

// ---------------------------------------------------------------------------
// cleanupStaleTerminals
// ---------------------------------------------------------------------------

func TestCleanupStaleTerminals_RemovesDeadSessions(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batched check outputs names of dead sessions (format: "<id> dead").
	mock.onCmd("for d in", "dead-wt dead\nanother-dead dead\n")
	// rm -rf succeeds. Use "rm -rf" to avoid matching "rm" in path strings
	// like ".warden/terminals" which contains the substring "rm".
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)

	removed := ec.cleanupStaleTerminals(context.Background(), "ctr-stale")

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d: %v", len(removed), removed)
	}

	// Verify rm -rf was called for both terminals.
	var rmCall *execCall
	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 3 && call.Cmd[0] == "sh" && call.Cmd[1] == "-c" {
			if strings.Contains(call.Cmd[2], "rm -rf") {
				rmCall = &call
				break
			}
		}
	}
	if rmCall == nil {
		t.Fatal("expected rm -rf exec call for stale terminals")
		return // unreachable but helps staticcheck
	}
	shellCmd := rmCall.Cmd[2]
	if !strings.Contains(shellCmd, "dead-wt") {
		t.Errorf("expected rm command to include 'dead-wt', got: %s", shellCmd)
	}
	if !strings.Contains(shellCmd, "another-dead") {
		t.Errorf("expected rm command to include 'another-dead', got: %s", shellCmd)
	}
}

func TestCleanupStaleTerminals_KillsOrphanedSessions(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batched check: abduco alive but worktree directory gone.
	mock.onCmd("for d in", "orphan-wt orphan\n")
	// kill-worktree.sh for the orphaned session.
	mock.onCmd(killWorktreeScript, "")
	// rm -rf for terminal dir cleanup.
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)

	removed := ec.cleanupStaleTerminals(context.Background(), "ctr-orphan")

	if len(removed) != 1 || removed[0] != "orphan-wt" {
		t.Fatalf("expected [orphan-wt], got %v", removed)
	}

	// Verify kill script was called for the orphaned worktree.
	var hasKill bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == killWorktreeScript {
			hasKill = true
		}
	}
	if !hasKill {
		t.Error("expected kill-worktree.sh to be called for orphaned worktree")
	}
}

func TestCleanupStaleTerminals_SkipsLiveSessions(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batched check outputs nothing (all sessions alive with existing dirs).
	mock.onCmd("for d in", "")

	ec := newTestClient(mock)

	removed := ec.cleanupStaleTerminals(context.Background(), "ctr-alive")

	if len(removed) != 0 {
		t.Errorf("expected no removals for live sessions, got %v", removed)
	}

	// Verify no rm -rf was called — only the batch check exec.
	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 exec call (batch check), got %d", len(calls))
	}
}

func TestCleanupStaleTerminals_EmptyDir(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// Batched check outputs nothing (no dirs).
	mock.onCmd("for d in", "")

	ec := newTestClient(mock)

	removed := ec.cleanupStaleTerminals(context.Background(), "ctr-empty")

	if len(removed) != 0 {
		t.Errorf("expected no removals for empty dir, got %v", removed)
	}

	// Only the batch check call should have been made.
	calls := mock.getCalls()
	if len(calls) != 1 {
		t.Errorf("expected 1 exec call (batch check), got %d", len(calls))
	}
}

// ---------------------------------------------------------------------------
// cleanupOrphanedWorktreeDirs
// ---------------------------------------------------------------------------

func TestCleanupOrphanedWorktreeDirs_RemovesUntracked(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// git worktree list returns only main.
	mock.onCmd("git", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n")
	// ls .claude/worktrees/ returns an orphan.
	mock.onCmd("ls", "orphan-wt\n")
	// rm succeeds.
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)
	primeGitRepoCache(ec, "ctr-orphan", true)

	removed, err := ec.cleanupOrphanedWorktreeDirs(context.Background(), "ctr-orphan")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed) != 1 || removed[0] != "orphan-wt" {
		t.Errorf("expected [orphan-wt], got %v", removed)
	}
}

func TestCleanupOrphanedWorktreeDirs_SkipsTracked(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// git worktree list reports feature-x.
	mock.onCmd("git", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n\nworktree /project/.claude/worktrees/feature-x\nHEAD def456\nbranch refs/heads/feature-x\n")
	// ls .claude/worktrees/ also shows feature-x — it's tracked, not orphaned.
	mock.onCmd("ls", "feature-x\n")

	ec := newTestClient(mock)
	primeGitRepoCache(ec, "ctr-tracked", true)

	removed, err := ec.cleanupOrphanedWorktreeDirs(context.Background(), "ctr-tracked")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed) != 0 {
		t.Errorf("expected no removals for tracked worktree, got %v", removed)
	}
}

// ---------------------------------------------------------------------------
// CleanupOrphanedWorktrees (integration of all three steps)
// ---------------------------------------------------------------------------

func TestCleanupOrphanedWorktrees_RunsAllSteps(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// git worktree prune + git worktree list both match "git".
	mock.onCmd("git", "worktree /project\nHEAD abc123\nbranch refs/heads/main\n")
	// ls matches both .claude/worktrees/ and .warden/terminals/ queries.
	mock.onCmd("ls", "")
	// pgrep for abduco check.
	mock.onCmd("pgrep", "0\n")

	ec := newTestClient(mock)
	primeGitRepoCache(ec, "ctr-full", true)

	_, err := ec.CleanupOrphanedWorktrees(context.Background(), "ctr-full")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify git worktree prune was called.
	var hasPrune bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 3 && call.Cmd[0] == "git" {
			cmd := strings.Join(call.Cmd, " ")
			if strings.Contains(cmd, "prune") {
				hasPrune = true
			}
		}
	}
	if !hasPrune {
		t.Error("expected git worktree prune call")
	}
}

func TestCleanupOrphanedWorktrees_SkipsNonGitRepos(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()

	ec := newTestClient(mock)
	primeGitRepoCache(ec, "ctr-nongit", false)

	removed, err := ec.CleanupOrphanedWorktrees(context.Background(), "ctr-nongit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if removed != nil {
		t.Errorf("expected nil removals for non-git repo, got %v", removed)
	}

	// No exec calls should have been made.
	if len(mock.getCalls()) != 0 {
		t.Errorf("expected no exec calls for non-git repo, got %d", len(mock.getCalls()))
	}
}

// ---------------------------------------------------------------------------
// RemoveWorktree
// ---------------------------------------------------------------------------

func TestRemoveWorktree_KillsAndRemoves(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// kill-worktree.sh runs unconditionally.
	mock.onCmd(killWorktreeScript, "")
	// git worktree prune + git worktree remove succeed.
	mock.onCmd("git", "")
	// rm cleanup succeeds.
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)

	err := ec.RemoveWorktree(context.Background(), "ctr-rm", "feature-x")
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify kill script was called.
	var hasKill, hasPrune, hasRemove bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == killWorktreeScript {
			hasKill = true
		}
		if len(call.Cmd) >= 3 && call.Cmd[0] == "git" {
			cmd := strings.Join(call.Cmd, " ")
			if strings.Contains(cmd, "prune") {
				hasPrune = true
			}
			if strings.Contains(cmd, "remove") && strings.Contains(cmd, "feature-x") {
				hasRemove = true
			}
		}
	}
	if !hasKill {
		t.Error("expected kill-worktree.sh to be called")
	}
	if !hasPrune {
		t.Error("expected git worktree prune before remove")
	}
	if !hasRemove {
		t.Error("expected git worktree remove call")
	}
}

func TestRemoveWorktree_ToleratesAlreadyRemovedByAgent(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// kill-worktree.sh runs.
	mock.onCmd(killWorktreeScript, "")
	// git worktree prune + remove both return (execAndCapture ignores exit codes).
	mock.onCmd("git", "")
	// rm cleanup for terminal dir.
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)

	// Should succeed even though git worktree was already removed by Claude.
	err := ec.RemoveWorktree(context.Background(), "ctr-orphan", "gone-wt")
	if err != nil {
		t.Fatalf("RemoveWorktree should tolerate already-removed worktrees, got: %v", err)
	}

	// Verify terminal dir cleanup still ran.
	var hasRmCleanup bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) >= 1 && call.Cmd[0] == "rm" {
			hasRmCleanup = true
		}
	}
	if !hasRmCleanup {
		t.Error("expected terminal dir cleanup even when git worktree was already gone")
	}
}

func TestRemoveWorktree_AlwaysCallsKill(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// kill-worktree.sh runs unconditionally (even if abduco is dead).
	mock.onCmd(killWorktreeScript, "")
	// git worktree remove succeeds.
	mock.onCmd("git", "")
	// rm cleanup succeeds.
	mock.onCmd("rm -rf", "")

	ec := newTestClient(mock)

	err := ec.RemoveWorktree(context.Background(), "ctr-rm2", "dead-wt")
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify kill script was called (unconditional, no pgrep check).
	var hasKill bool
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == killWorktreeScript {
			hasKill = true
		}
	}
	if !hasKill {
		t.Error("expected kill-worktree.sh to be called unconditionally")
	}
}

func TestRemoveWorktree_RejectsMain(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	ec := newTestClient(mock)

	err := ec.RemoveWorktree(context.Background(), "ctr-rm3", "main")
	if err == nil {
		t.Fatal("expected error when removing main worktree")
	}
}

func TestRemoveWorktree_RejectsInvalidID(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	ec := newTestClient(mock)

	err := ec.RemoveWorktree(context.Background(), "ctr-rm4", "../etc/passwd")
	if err == nil {
		t.Fatal("expected error for invalid worktree ID")
	}
}

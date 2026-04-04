package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// CleanupOrphanedWorktrees tidies up worktree-related state in the container:
//
//  1. Prunes git worktree metadata for directories that no longer exist on disk.
//  2. Removes .claude/worktrees/ directories not tracked by git (leftovers from
//     kill-worktree.sh running before Claude could call `git worktree remove`).
//  3. Removes .warden/terminals/<id>/ directories whose tmux sessions are dead
//     OR whose worktree directories no longer exist on disk (orphaned by Claude
//     running `git worktree remove` inside the container). Kills orphaned tmux
//     sessions before removing their tracking directories.
//
// Returns the deduplicated list of removed worktree IDs from steps 2 and 3.
func (ec *EngineClient) CleanupOrphanedWorktrees(ctx context.Context, containerID string) ([]string, error) {
	if !ec.checkIsGitRepo(ctx, containerID) {
		return nil, nil
	}

	// Step 1: Prune git metadata for worktrees whose directories are gone.
	ec.pruneGitWorktrees(ctx, containerID)

	// Step 2: Remove .claude/worktrees/ directories not tracked by git.
	orphans, err := ec.cleanupOrphanedWorktreeDirs(ctx, containerID)
	if err != nil {
		return nil, err
	}

	// Step 3: Remove .warden/terminals/<id>/ directories with no live tmux session
	// OR whose worktree directory no longer exists.
	staleTerminals := ec.cleanupStaleTerminals(ctx, containerID)

	// Merge stale terminal IDs into the removed list (deduplicated).
	seen := make(map[string]bool, len(orphans))
	for _, id := range orphans {
		seen[id] = true
	}
	for _, id := range staleTerminals {
		if !seen[id] {
			orphans = append(orphans, id)
			seen[id] = true
		}
	}

	return orphans, nil
}

// pruneGitWorktrees runs `git worktree prune` to remove git metadata for
// worktrees whose directories no longer exist on disk.
func (ec *EngineClient) pruneGitWorktrees(ctx context.Context, containerID string) {
	wsDir := ec.workspaceDir(ctx, containerID)
	_, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"git", "-C", wsDir, "-c", "safe.directory=" + wsDir, "worktree", "prune"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Warn("git worktree prune failed", "container", containerID, "err", err)
	}
}

// cleanupOrphanedWorktreeDirs removes directories from the worktree directories
// (.claude/worktrees/ and .warden/worktrees/) that are not tracked by git.
func (ec *EngineClient) cleanupOrphanedWorktreeDirs(ctx context.Context, containerID string) ([]string, error) {
	gitWorktrees, err := ec.discoverGitWorktrees(ctx, containerID)
	if err != nil {
		return nil, fmt.Errorf("discovering git worktrees: %w", err)
	}

	known := make(map[string]bool, len(gitWorktrees))
	for _, wt := range gitWorktrees {
		known[wt.ID] = true
	}

	wsDir := ec.workspaceDir(ctx, containerID)
	prefixes := []string{
		wsDir + claudeWorktreesPrefixSuffix,
		wsDir + wardenWorktreesPrefixSuffix,
	}

	seen := make(map[string]bool)
	var allOrphans []string
	for _, prefix := range prefixes {
		entries := ec.listDirEntries(ctx, containerID, prefix)
		var orphansForPrefix []string
		for _, name := range entries {
			if known[name] || seen[name] {
				continue
			}
			seen[name] = true
			orphansForPrefix = append(orphansForPrefix, name)
		}
		if len(orphansForPrefix) == 0 {
			continue
		}
		if err := ec.removeDirs(ctx, containerID, prefix, orphansForPrefix); err != nil {
			slog.Warn("failed to remove orphaned worktrees", "container", containerID, "err", err)
		}
		for _, name := range orphansForPrefix {
			slog.Info("removed orphaned worktree directory", "container", containerID, "worktree", name)
		}
		allOrphans = append(allOrphans, orphansForPrefix...)
	}

	return allOrphans, nil
}

// cleanupStaleTerminals removes .warden/terminals/<id>/ directories that are
// stale. A terminal is stale when either:
//   - The tmux session is dead (normal exit, Ctrl-C, container restart).
//   - The worktree directory no longer exists on disk (removed by Claude inside
//     the container without Warden knowing).
//
// For orphaned terminals with a live tmux session but no worktree directory,
// the session is killed before removing the tracking directory.
//
// Returns the list of cleaned-up worktree IDs so the caller can evict
// their cached state from the event store.
func (ec *EngineClient) cleanupStaleTerminals(ctx context.Context, containerID string) []string {
	wsDir := ec.workspaceDir(ctx, containerID)
	termDir := wsDir + terminalsDirSuffix
	claudePrefix := wsDir + claudeWorktreesPrefixSuffix
	wardenPrefix := wsDir + wardenWorktreesPrefixSuffix

	// Single exec: list all tmux sessions, then for each terminal dir, check
	// session liveness and worktree directory existence. Print "<id> dead"
	// if the session is dead, or "<id> orphan" if the session is alive but
	// no worktree dir exists.
	cmd := fmt.Sprintf(
		`TMUX_SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null || true); `+
			`for d in %s/*/; do [ -d "$d" ] || continue; id=$(basename "$d"); `+
			`alive=0; echo "$TMUX_SESSIONS" | grep -qx "warden-$id" && alive=1; `+
			`has_dir=0; { [ -d "%s$id" ] || [ -d "%s$id" ] || [ "$id" = "main" ]; } && has_dir=1; `+
			`if [ "$alive" = "0" ]; then echo "$id dead"; `+
			`elif [ "$has_dir" = "0" ]; then echo "$id orphan"; fi; done`,
		termDir, claudePrefix, wardenPrefix,
	)
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	var stale, orphans []string
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, " ", 2)
		name := parts[0]
		kind := ""
		if len(parts) > 1 {
			kind = parts[1]
		}
		switch kind {
		case "orphan":
			orphans = append(orphans, name)
		default:
			stale = append(stale, name)
		}
	}

	// Kill tmux sessions for orphaned worktrees (live session, no directory).
	for _, name := range orphans {
		if killErr := ec.KillWorktreeProcess(ctx, containerID, name); killErr != nil {
			slog.Warn("failed to kill orphaned worktree process", "container", containerID, "worktree", name, "err", killErr)
		} else {
			slog.Info("killed orphaned worktree process", "container", containerID, "worktree", name)
		}
	}

	all := append(stale, orphans...)
	if len(all) == 0 {
		return nil
	}

	if err := ec.removeDirs(ctx, containerID, termDir, all); err != nil {
		slog.Warn("failed to remove stale terminal directories", "container", containerID, "err", err)
		// Still return IDs — tmux was already killed for orphans, so callers
		// need these to evict cached state even if dir removal failed.
		return all
	}

	for _, name := range all {
		slog.Info("removed stale terminal directory", "container", containerID, "worktree", name)
	}

	return all
}

// removeDirs removes directories under a base path inside the container.
func (ec *EngineClient) removeDirs(ctx context.Context, containerID, baseDir string, names []string) error {
	var rmParts []string
	for _, name := range names {
		rmParts = append(rmParts, fmt.Sprintf("rm -rf '%s/%s'", baseDir, name))
	}

	_, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", strings.Join(rmParts, " ; ")},
		AttachStdout: true,
		AttachStderr: true,
	})
	return err
}

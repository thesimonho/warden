package engine

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/docker/docker/api/types/container"
)

// DisconnectTerminal pushes a disconnect event and cleans up tracking state.
// The tmux session (and Claude/bash running inside it) continues in the background.
func (ec *EngineClient) DisconnectTerminal(ctx context.Context, containerID, worktreeID string) error {
	if !IsValidWorktreeID(worktreeID) {
		return fmt.Errorf("invalid worktree ID: %q", worktreeID)
	}

	_, err := ec.execAndCaptureStrict(ctx, containerID, container.ExecOptions{
		Cmd:          []string{disconnectTerminalScript, worktreeID},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("disconnecting terminal: %w", err)
	}

	slog.Info("disconnected terminal", "container", containerID, "worktree", worktreeID)
	return nil
}

// RemoveWorktree fully removes a worktree: kills any running tmux session,
// runs `git worktree remove --force`, and cleans up the .warden/terminals/
// tracking directory. Cannot remove the "main" worktree.
//
// Tolerates missing git worktrees (e.g. when Claude already ran
// `git worktree remove` inside the container). In that case, git metadata
// is pruned and the terminal tracking directory is still cleaned up.
func (ec *EngineClient) RemoveWorktree(ctx context.Context, containerID, worktreeID string) error {
	if !IsValidWorktreeID(worktreeID) {
		return fmt.Errorf("invalid worktree ID: %q", worktreeID)
	}
	if worktreeID == "main" {
		return fmt.Errorf("cannot remove the main worktree")
	}

	// Kill tmux session unconditionally — may already be dead.
	if killErr := ec.KillWorktreeProcess(ctx, containerID, worktreeID); killErr != nil {
		slog.Debug("kill before remove failed (session may already be dead)", "container", containerID, "worktree", worktreeID, "err", killErr)
	}

	// Prune stale git metadata first — if Claude already removed the worktree
	// directory, this marks it as gone so the subsequent remove doesn't fail.
	ec.pruneGitWorktrees(ctx, containerID)

	// Remove the git worktree by full path (git expects a path, not a name).
	// --force handles dirty working trees. Tolerate errors: the worktree may
	// already be gone (removed by the agent or pruned above).
	worktreePath := ec.resolveWorktreePath(ctx, containerID, worktreeID)
	wsDir := ec.workspaceDir(ctx, containerID)
	_, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"git", "-C", wsDir, "-c", "safe.directory=" + wsDir, "worktree", "remove", "--force", worktreePath},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Warn("git worktree remove failed (may already be removed)", "container", containerID, "worktree", worktreeID, "err", err)
	}

	// Clean up tracking directory (may already be gone from KillWorktreeProcess).
	termDir := wsDir + terminalsDirSuffix
	if _, rmErr := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"rm", "-rf", fmt.Sprintf("%s/%s", termDir, worktreeID)},
		AttachStdout: true,
		AttachStderr: true,
	}); rmErr != nil {
		slog.Warn("failed to clean up worktree tracking dir", "container", containerID, "worktree", worktreeID, "err", rmErr)
	}

	slog.Info("removed worktree", "container", containerID, "worktree", worktreeID)
	return nil
}

// ResetWorktree clears all history for a worktree without removing it from
// disk. It kills any running tmux session, removes the terminal tracking
// directory (exit_code, inner-cmd.sh), and deletes agent JSONL session files
// so that the FileTailer won't replay old events on restart.
func (ec *EngineClient) ResetWorktree(ctx context.Context, containerID, worktreeID string) error {
	if !IsValidWorktreeID(worktreeID) {
		return fmt.Errorf("invalid worktree ID: %q", worktreeID)
	}

	// Kill tmux session — may already be dead.
	if killErr := ec.KillWorktreeProcess(ctx, containerID, worktreeID); killErr != nil {
		slog.Debug("kill before reset failed (session may already be dead)", "container", containerID, "worktree", worktreeID, "err", killErr)
	}

	// Remove terminal tracking directory.
	wsDir := ec.workspaceDir(ctx, containerID)
	termDir := wsDir + terminalsDirSuffix
	if _, rmErr := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"rm", "-rf", fmt.Sprintf("%s/%s", termDir, worktreeID)},
		AttachStdout: true,
		AttachStderr: true,
	}); rmErr != nil {
		slog.Warn("failed to clean up terminal tracking dir", "container", containerID, "worktree", worktreeID, "err", rmErr)
	}

	// Clear agent JSONL session files so they won't be replayed.
	ec.clearSessionFiles(ctx, containerID)

	slog.Info("reset worktree", "container", containerID, "worktree", worktreeID)
	return nil
}

// clearSessionFiles removes agent JSONL session files from the container.
// Both agent types are cleaned in a single exec — only one will have files.
func (ec *EngineClient) clearSessionFiles(ctx context.Context, containerID string) {
	h := ContainerHomeDir
	cmd := fmt.Sprintf(
		"find %s/.claude/projects -name '*.jsonl' -delete 2>/dev/null; "+
			"find %s/.codex/sessions -name '*.jsonl' -delete 2>/dev/null; "+
			"rm -rf %s/.codex/shell_snapshots 2>/dev/null; true",
		h, h, h,
	)
	if _, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	}); err != nil {
		slog.Debug("clearing session files", "container", containerID, "err", err)
	}
}

// KillWorktreeProcess kills the tmux session for a worktree, destroying the
// process entirely. The git worktree directory on disk is preserved.
// Use DisconnectTerminal to only disconnect the viewer and keep the session alive.
func (ec *EngineClient) KillWorktreeProcess(ctx context.Context, containerID, worktreeID string) error {
	if !IsValidWorktreeID(worktreeID) {
		return fmt.Errorf("invalid worktree ID: %q", worktreeID)
	}

	_, err := ec.execAndCaptureStrict(ctx, containerID, container.ExecOptions{
		Cmd:          []string{killWorktreeScript, worktreeID},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("killing worktree process: %w", err)
	}

	slog.Info("killed worktree process", "container", containerID, "worktree", worktreeID)
	return nil
}

// resolveWorktreePath returns the filesystem path for a worktree inside a container.
// For Claude Code, worktrees live under .claude/worktrees/; for other agents
// (Codex, future), Warden manages worktrees under .warden/worktrees/.
func (ec *EngineClient) resolveWorktreePath(ctx context.Context, containerID, worktreeID string) string {
	wsDir := ec.workspaceDir(ctx, containerID)
	if worktreeID == "main" {
		return wsDir
	}
	return wsDir + ec.worktreesPrefixSuffix(ctx, containerID) + worktreeID
}

// worktreesPrefixSuffix returns the worktree directory suffix for the container's agent type.
func (ec *EngineClient) worktreesPrefixSuffix(ctx context.Context, containerID string) string {
	agentType := ec.cachedAgentType(ctx, containerID)
	if agentType == "claude-code" || agentType == "" {
		return claudeWorktreesPrefixSuffix
	}
	return wardenWorktreesPrefixSuffix
}

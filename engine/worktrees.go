package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
)

// ContainerUser is the non-root user inside project containers.
const ContainerUser = constants.ContainerUser

// ContainerHomeDir is the home directory for [ContainerUser] inside containers.
const ContainerHomeDir = constants.ContainerHomeDir

// TmuxSessionName returns the tmux session name for a worktree.
var TmuxSessionName = constants.TmuxSessionName

// createTerminalScript is the path to the terminal creator inside the container.
const createTerminalScript = "/usr/local/bin/create-terminal.sh"

// disconnectTerminalScript pushes a disconnect event and cleans up tracking state.
// The tmux session and everything inside it continues running.
const disconnectTerminalScript = "/usr/local/bin/disconnect-terminal.sh"

// killWorktreeScript kills the tmux session — the worktree process is fully destroyed.
const killWorktreeScript = "/usr/local/bin/kill-worktree.sh"

// terminalsDirSuffix is appended to the workspace dir for terminal tracking.
const terminalsDirSuffix = "/.warden/terminals"

// claudeWorktreesPrefixSuffix is Claude Code's worktree path suffix.
const claudeWorktreesPrefixSuffix = "/.claude/worktrees/"

// wardenWorktreesPrefixSuffix is Warden's worktree path for non-Claude agents (Codex, future).
const wardenWorktreesPrefixSuffix = "/.warden/worktrees/"

// IsValidWorktreeID validates worktree IDs (alphanumeric start, then alphanumeric/hyphens/underscores/dots).
func IsValidWorktreeID(id string) bool {
	if id == "" {
		return false
	}
	for i, c := range id {
		if i == 0 {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
				return false
			}
		} else {
			if (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') && c != '-' && c != '_' && c != '.' {
				return false
			}
		}
	}
	return true
}

// CreateWorktree creates a new worktree terminal inside the container.
// Claude Code's --worktree flag handles git worktree creation and isolation,
// so the worktree won't exist in git yet — we skip the orphan check.
// When skipPermissions is true, Claude Code runs with --dangerously-skip-permissions.
func (ec *EngineClient) CreateWorktree(ctx context.Context, containerID, name string, skipPermissions bool) (string, error) {
	if !IsValidWorktreeID(name) {
		return "", fmt.Errorf("invalid worktree name: %q", name)
	}

	return ec.connectTerminal(ctx, containerID, name, skipPermissions, true)
}

// ConnectTerminal starts a terminal for a worktree inside the container.
// If a tmux session is still alive (background state), the WebSocket proxy
// will attach to it. Otherwise runs create-terminal.sh which starts a tmux
// session and launches the agent.
// When skipPermissions is true, Claude Code runs with --dangerously-skip-permissions.
func (ec *EngineClient) ConnectTerminal(ctx context.Context, containerID, worktreeID string, skipPermissions bool) (string, error) {
	if !IsValidWorktreeID(worktreeID) {
		return "", fmt.Errorf("invalid worktree ID: %q", worktreeID)
	}

	return ec.connectTerminal(ctx, containerID, worktreeID, skipPermissions, false)
}

// connectTerminal is the shared implementation for CreateWorktree and ConnectTerminal.
// When skipPermissions is true, Claude Code runs with --dangerously-skip-permissions.
// When isCreate is true, the git worktree orphan check is skipped because Claude
// Code's --worktree flag will create the worktree.
func (ec *EngineClient) connectTerminal(ctx context.Context, containerID, worktreeID string, skipPermissions, isCreate bool) (string, error) {
	// Check if a tmux session is still alive (background state)
	isBackground := ec.isSessionAlive(ctx, containerID, worktreeID)

	// For reconnects (not creates), verify the worktree exists in git.
	// Prevents launching a broken terminal for orphaned worktree directories
	// that git no longer tracks.
	if !isCreate && !isBackground && worktreeID != "main" && ec.checkIsGitRepo(ctx, containerID) {
		if !ec.isGitWorktreeKnown(ctx, containerID, worktreeID) {
			return "", fmt.Errorf("worktree %q is not tracked by git — it may be an orphaned directory", worktreeID)
		}
	}

	// For background state, the tmux session is already running. The WebSocket
	// proxy will attach to it via docker exec — no script needed. Return early
	// so the frontend knows it can open a WebSocket immediately.
	if isBackground {
		slog.Info("reconnected terminal", "container", containerID, "worktree", worktreeID)
		return worktreeID, nil
	}

	cmd := []string{createTerminalScript, worktreeID}
	if skipPermissions {
		cmd = append(cmd, "--skip-permissions")
	}

	output, err := ec.execAndCaptureStrict(ctx, containerID, container.ExecOptions{
		Cmd:          cmd,
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		// Provide a clearer message when the script is missing entirely (exit code 127)
		if strings.Contains(err.Error(), "status 127") || strings.Contains(err.Error(), "not found") {
			return "", fmt.Errorf("terminal infrastructure not installed in container (missing %s)", createTerminalScript)
		}
		return "", fmt.Errorf("creating terminal: %w", err)
	}

	var resp struct {
		WorktreeID string `json:"worktreeId"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &resp); err != nil {
		return "", fmt.Errorf("parsing terminal response: %w (output: %s)", err, output)
	}

	slog.Info("connected terminal", "container", containerID, "worktree", resp.WorktreeID)
	return resp.WorktreeID, nil
}

// isGitWorktreeKnown checks whether a worktree ID appears in git's worktree list.
// Returns false for orphaned directories that exist on disk but aren't tracked by git.
func (ec *EngineClient) isGitWorktreeKnown(ctx context.Context, containerID, worktreeID string) bool {
	worktrees, err := ec.discoverGitWorktrees(ctx, containerID)
	if err != nil {
		return true // fail open — don't block connections on git errors
	}
	for _, wt := range worktrees {
		if wt.ID == worktreeID {
			return true
		}
	}
	return false
}

// isSessionAlive checks if a tmux session for the worktree is running.
// Uses `tmux has-session` which returns exit code 0 if the session exists.
func (ec *EngineClient) isSessionAlive(ctx context.Context, containerID, worktreeID string) bool {
	sessionName := TmuxSessionName(worktreeID)
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", fmt.Sprintf(`tmux has-session -t "%s" 2>/dev/null && echo 1 || echo 0`, sessionName)},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "1"
}

// ListWorktrees returns all worktrees for the given container with their terminal state.
// For git repos, discovers worktrees via `git worktree list --porcelain`.
// For non-git repos, returns a single implicit worktree at /project.
// When skipEnrich is true, the expensive batch docker exec for terminal state is skipped.
func (ec *EngineClient) ListWorktrees(ctx context.Context, containerID string, skipEnrich bool) ([]Worktree, error) {
	return ec.listWorktreesWithHint(ctx, containerID, ec.checkIsGitRepo(ctx, containerID), skipEnrich)
}

// listWorktreesWithHint is the internal implementation of ListWorktrees that accepts
// a pre-computed isGitRepo flag to avoid a duplicate exec when the caller already knows.
// When skipEnrich is true, the expensive batch exec for terminal state is skipped.
func (ec *EngineClient) listWorktreesWithHint(ctx context.Context, containerID string, isGitRepo, skipEnrich bool) ([]Worktree, error) {
	var worktrees []Worktree
	if isGitRepo {
		var err error
		worktrees, err = ec.discoverGitWorktrees(ctx, containerID)
		if err != nil {
			return nil, fmt.Errorf("discovering worktrees: %w", err)
		}
	} else {
		worktrees = []Worktree{
			{
				ID:        "main",
				ProjectID: containerID,
				Path:      ec.workspaceDir(ctx, containerID),
				State:     WorktreeStateStopped,
			},
		}
	}

	// Discover terminals that have tracking directories but no corresponding
	// git worktree yet (e.g. Claude Code is still creating the worktree).
	ec.mergeTerminalWorktrees(ctx, containerID, &worktrees)

	// Enrich worktrees with terminal state from .warden/terminals/
	// Skip when the caller will overlay state from the event bus store.
	if !skipEnrich {
		ec.enrichWorktreeState(ctx, containerID, worktrees)
	}

	return worktrees, nil
}

// listDirEntries lists non-hidden entries in a container directory.
// Returns nil when the directory is empty or doesn't exist.
func (ec *EngineClient) listDirEntries(ctx context.Context, containerID, dir string) []string {
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", fmt.Sprintf("ls -1 %s 2>/dev/null", dir)},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil || strings.TrimSpace(output) == "" {
		return nil
	}

	var entries []string
	for _, name := range strings.Split(strings.TrimSpace(output), "\n") {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		entries = append(entries, name)
	}
	return entries
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

// mergeTerminalWorktrees lists directories under .warden/terminals/ and adds
// any that are not already represented in the worktree list. This ensures
// worktrees whose terminals launched before the agent created the git worktree
// still appear in the UI.
func (ec *EngineClient) mergeTerminalWorktrees(ctx context.Context, containerID string, worktrees *[]Worktree) {
	wsDir := ec.workspaceDir(ctx, containerID)
	termDir := wsDir + terminalsDirSuffix
	entries := ec.listDirEntries(ctx, containerID, termDir)

	known := make(map[string]bool, len(*worktrees))
	for _, wt := range *worktrees {
		known[wt.ID] = true
	}

	prefix := wsDir + ec.worktreesPrefixSuffix(ctx, containerID)
	for _, name := range entries {
		if known[name] {
			continue
		}
		*worktrees = append(*worktrees, Worktree{
			ID:        name,
			ProjectID: containerID,
			Path:      prefix + name,
			State:     WorktreeStateStopped,
		})
	}
}

// discoverGitWorktrees parses `git worktree list --porcelain` output
// to discover all worktrees in the container.
func (ec *EngineClient) discoverGitWorktrees(ctx context.Context, containerID string) ([]Worktree, error) {
	wsDir := ec.workspaceDir(ctx, containerID)
	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"git", "-C", wsDir, "-c", "safe.directory=" + wsDir, "worktree", "list", "--porcelain"},
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("running git worktree list: %w", err)
	}

	return parseGitWorktreeList(containerID, output, wsDir), nil
}

// parseGitWorktreeList parses the porcelain output of `git worktree list`.
// Format: blocks separated by blank lines, each block has:
//
//	worktree <path>
//	HEAD <sha>
//	branch refs/heads/<name>
//	prunable <reason>  (optional — present when git considers the worktree stale)
func parseGitWorktreeList(containerID, output, wsDir string) []Worktree {
	var worktrees []Worktree

	// Split into blocks by blank lines
	blocks := strings.Split(output, "\n\n")
	for _, block := range blocks {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}

		var path, branch string
		isPrunable := false
		scanner := bufio.NewScanner(strings.NewReader(block))
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "worktree "):
				path = strings.TrimPrefix(line, "worktree ")
			case strings.HasPrefix(line, "branch "):
				branch = strings.TrimPrefix(line, "branch ")
				// Strip refs/heads/ prefix
				branch = strings.TrimPrefix(branch, "refs/heads/")
			case strings.HasPrefix(line, "prunable "):
				isPrunable = true
			}
		}

		if path == "" {
			continue
		}

		// Skip stale worktrees that git has flagged for pruning.
		// These have a broken gitdir reference (the worktree directory was
		// deleted without running `git worktree remove`) and should not be
		// shown in the UI since they represent phantom state.
		if isPrunable {
			continue
		}

		// Derive worktree ID from path
		id := worktreeIDFromPath(path, wsDir)

		worktrees = append(worktrees, Worktree{
			ID:        id,
			ProjectID: containerID,
			Path:      path,
			Branch:    branch,
			State:     WorktreeStateStopped, // default, enriched later
		})
	}

	return worktrees
}

// worktreeIDFromPath extracts the worktree ID from its filesystem path.
// <wsDir> → "main"
// <wsDir>/.claude/worktrees/feature-x → "feature-x" (Claude Code)
// <wsDir>/.warden/worktrees/feature-x → "feature-x" (Warden-managed)
func worktreeIDFromPath(path, wsDir string) string {
	claudePrefix := wsDir + claudeWorktreesPrefixSuffix
	if strings.HasPrefix(path, claudePrefix) {
		return strings.TrimPrefix(path, claudePrefix)
	}
	wardenPrefix := wsDir + wardenWorktreesPrefixSuffix
	if strings.HasPrefix(path, wardenPrefix) {
		return strings.TrimPrefix(path, wardenPrefix)
	}
	return "main"
}

// enrichWorktreeState reads terminal tracking data from .warden/terminals/
// to determine each worktree's terminal state (liveness, exit code).
// Attention state is handled separately via the event bus push path.
func (ec *EngineClient) enrichWorktreeState(ctx context.Context, containerID string, worktrees []Worktree) {
	if len(worktrees) == 0 {
		return
	}

	// Build a batch command to read terminal state for all worktrees.
	// First lists all tmux sessions, then checks exit code and session
	// liveness for each worktree using grep against the session list.
	//
	// Attention state is NOT read here — it's handled by the event bus push path.
	wsDir := ec.workspaceDir(ctx, containerID)
	termDir := wsDir + terminalsDirSuffix

	var cmdParts []string
	// Prepend a single tmux list-sessions call and store the result.
	cmdParts = append(cmdParts, `TMUX_SESSIONS=$(tmux list-sessions -F '#{session_name}' 2>/dev/null || true)`)
	for _, wt := range worktrees {
		cmdParts = append(cmdParts,
			fmt.Sprintf(
				`echo "---WT_START:%s---" && (cat %s/%s/exit_code 2>/dev/null || true) && echo "---EXIT_END---" && echo "$TMUX_SESSIONS" | grep -qx "warden-%s" && echo 1 || echo 0 && echo "---SESSION_END---"`,
				wt.ID, termDir, wt.ID, wt.ID,
			),
		)
	}

	batchOutput, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", strings.Join(cmdParts, " ; ")},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		slog.Debug("failed to read terminal state", "container", containerID, "err", err)
		return
	}

	// Parse the batch output and update worktree states.
	terminalStates := parseTerminalBatch(batchOutput)

	for i := range worktrees {
		ts, ok := terminalStates[worktrees[i].ID]
		if !ok {
			continue
		}

		if ts.sessionAlive {
			if ts.exitCode >= 0 {
				// Agent exited but shell is still alive
				worktrees[i].State = WorktreeStateShell
				code := ts.exitCode
				worktrees[i].ExitCode = &code
			} else {
				worktrees[i].State = WorktreeStateConnected
			}
		}
	}
}

// terminalState holds parsed terminal tracking data for a worktree.
type terminalState struct {
	exitCode     int // -1 means not set (Claude still running)
	sessionAlive bool
}

// parseTerminalBatch parses the batched output from the terminal state read command.
func parseTerminalBatch(output string) map[string]*terminalState {
	states := make(map[string]*terminalState)
	blocks := strings.Split(output, "---WT_START:")

	for _, block := range blocks {
		if block == "" {
			continue
		}

		headerEnd := strings.Index(block, "---")
		if headerEnd < 0 {
			continue
		}
		worktreeID := block[:headerEnd]
		rest := block[headerEnd+3:]

		ts := &terminalState{exitCode: -1}

		// Parse exit code
		if exitEnd := strings.Index(rest, "---EXIT_END---"); exitEnd >= 0 {
			exitStr := strings.TrimSpace(rest[:exitEnd])
			if code, err := strconv.Atoi(exitStr); err == nil {
				ts.exitCode = code
			}
			rest = rest[exitEnd+len("---EXIT_END---"):]
		}

		// Parse session alive check
		if sessionEnd := strings.Index(rest, "---SESSION_END---"); sessionEnd >= 0 {
			ts.sessionAlive = strings.TrimSpace(rest[:sessionEnd]) == "1"
		}

		states[worktreeID] = ts
	}

	return states
}

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

// maxRawDiffBytes is the maximum size of the raw diff output before truncation.
const maxRawDiffBytes = 1 << 20 // 1 MB

// diffSeparator delimits numstat output from the raw unified diff.
const diffSeparator = "---WARDEN_DIFF_SEP---"

// untrackedMarker is appended to numstat lines for untracked files so
// parseNumstat can distinguish them from tracked modified files.
const untrackedMarker = "\t[untracked]"

// GetWorktreeDiff returns the uncommitted changes (tracked + untracked) for a
// worktree inside the container. Uses a temporary index copy so the real
// index is never modified.
func (ec *EngineClient) GetWorktreeDiff(ctx context.Context, containerID, worktreeID string) (*api.DiffResponse, error) {
	worktreePath := ec.resolveWorktreePath(ctx, containerID, worktreeID)

	// Single exec: copy the git index, intent-to-add untracked files on the
	// copy, then run numstat + unified diff. The awk script tags untracked
	// files with [untracked] in O(1) per line. It reads the untracked list
	// in BEGIN via getline to avoid the NR==FNR empty-file bug (when the
	// first file is empty, NR==FNR stays true for stdin lines too, causing
	// the entire numstat to be swallowed).
	cmd := fmt.Sprintf(
		`cd %[1]s 2>/dev/null || exit 0; `+
			`git rev-parse --git-dir >/dev/null 2>&1 || exit 0; `+
			`gitdir=$(git rev-parse --git-dir); `+
			`tmpidx=$(mktemp); `+
			`cp "$gitdir/index" "$tmpidx" 2>/dev/null || true; `+
			`uf=/tmp/warden_untracked$$; `+
			`git -c safe.directory=%[1]s ls-files --others --exclude-standard 2>/dev/null > "$uf"; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s add -N . 2>/dev/null; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s diff HEAD --numstat 2>/dev/null | `+
			`awk -v uf="$uf" 'BEGIN{while((getline line < uf)>0) u[line]; close(uf)} {f=$3; for(i=4;i<=NF;i++) f=f" "$i; if(f in u) print $0"\t[untracked]"; else print}'; `+
			`echo '%[2]s'; `+
			`GIT_INDEX_FILE="$tmpidx" git -c safe.directory=%[1]s diff HEAD 2>/dev/null; `+
			`rm -f "$tmpidx" "$uf"`,
		worktreePath, diffSeparator,
	)

	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		// Non-git repos or errors return empty response, not an error.
		slog.Debug("worktree diff failed", "container", containerID, "worktree", worktreeID, "err", err)
		return &api.DiffResponse{Files: []api.DiffFileSummary{}}, nil
	}

	return parseDiffOutput(output), nil
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

// parseDiffOutput splits the combined exec output into numstat + raw diff
// and returns a fully populated DiffResponse.
func parseDiffOutput(output string) *api.DiffResponse {
	resp := &api.DiffResponse{}

	parts := strings.SplitN(output, diffSeparator, 2)
	numstatSection := ""
	rawDiff := ""
	if len(parts) >= 1 {
		numstatSection = strings.TrimSpace(parts[0])
	}
	if len(parts) >= 2 {
		rawDiff = strings.TrimSpace(parts[1])
	}

	if files := parseNumstat(numstatSection); files != nil {
		resp.Files = files
	} else {
		resp.Files = []api.DiffFileSummary{}
	}

	// Truncate raw diff if too large.
	if len(rawDiff) > maxRawDiffBytes {
		rawDiff = rawDiff[:maxRawDiffBytes]
		resp.Truncated = true
	}
	resp.RawDiff = rawDiff

	for _, f := range resp.Files {
		resp.TotalAdditions += f.Additions
		resp.TotalDeletions += f.Deletions
	}

	return resp
}

// parseNumstat parses git diff --numstat output into file summaries.
//
// Format:
//
//	<add>\t<del>\t<path>          — normal file
//	-\t-\t<path>                  — binary file
//	<add>\t<del>\t{old => new}    — rename (various patterns)
//	<add>\t<del>\t<path>\t[untracked] — untracked file (Warden marker)
func parseNumstat(input string) []api.DiffFileSummary {
	input = strings.TrimSpace(input)
	if input == "" {
		return nil
	}

	var files []api.DiffFileSummary
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Check for untracked marker at the end.
		isUntracked := strings.HasSuffix(line, untrackedMarker)
		if isUntracked {
			line = strings.TrimSuffix(line, untrackedMarker)
		}

		parts := strings.SplitN(line, "\t", 3)
		if len(parts) < 3 {
			continue
		}

		addStr, delStr, path := parts[0], parts[1], parts[2]

		var f api.DiffFileSummary

		// Binary files show "-" for both add and delete counts.
		if addStr == "-" && delStr == "-" {
			f.Path = path
			f.IsBinary = true
			f.Status = "modified"
			files = append(files, f)
			continue
		}

		f.Additions, _ = strconv.Atoi(addStr)
		f.Deletions, _ = strconv.Atoi(delStr)

		// Detect renames: git uses {old => new} inside the path.
		if strings.Contains(path, " => ") && strings.Contains(path, "{") {
			f.Path, f.OldPath = parseRenamePath(path)
			f.Status = "renamed"
		} else {
			f.Path = path
			f.Status = deriveFileStatus(f.Additions, f.Deletions, isUntracked)
		}

		files = append(files, f)
	}

	return files
}

// deriveFileStatus infers the change type from line counts.
// Only the [untracked] marker reliably indicates a new file — a tracked file
// with additions-only is a modification, not an addition.
func deriveFileStatus(additions, deletions int, isUntracked bool) string {
	if isUntracked {
		return "added"
	}
	if additions == 0 && deletions > 0 {
		return "deleted"
	}
	return "modified"
}

// parseRenamePath extracts old and new paths from git's rename notation.
//
// Examples:
//
//	{old.txt => new.txt}            → new.txt, old.txt
//	src/{utils => helpers}/parse.go → src/helpers/parse.go, src/utils/parse.go
//	{old => new}/file.go            → new/file.go, old/file.go
func parseRenamePath(path string) (newPath, oldPath string) {
	braceStart := strings.Index(path, "{")
	braceEnd := strings.Index(path, "}")
	if braceStart < 0 || braceEnd < 0 || braceEnd <= braceStart {
		return path, ""
	}

	prefix := path[:braceStart]
	suffix := path[braceEnd+1:]
	inner := path[braceStart+1 : braceEnd]

	arrowIdx := strings.Index(inner, " => ")
	if arrowIdx < 0 {
		return path, ""
	}

	oldPart := inner[:arrowIdx]
	newPart := inner[arrowIdx+4:]

	return prefix + newPart + suffix, prefix + oldPart + suffix
}

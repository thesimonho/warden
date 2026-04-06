package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
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

// SendWorktreeInput sends text to a worktree's tmux pane. Uses `tmux send-keys -l`
// (literal mode) to prevent tmux key-name interpretation. If pressEnter is true,
// sends a separate Enter keystroke after the text.
func (ec *EngineClient) SendWorktreeInput(ctx context.Context, containerID, worktreeID, text string, pressEnter bool) error {
	sessionName := TmuxSessionName(worktreeID)

	// Check session exists first.
	if !ec.isSessionAlive(ctx, containerID, worktreeID) {
		return fmt.Errorf("no tmux session for worktree %q", worktreeID)
	}

	// Send literal text (no key-name interpretation).
	_, err := ec.execAndCaptureStrict(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"tmux", "send-keys", "-t", sessionName, "-l", text},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return fmt.Errorf("sending text to tmux session: %w", err)
	}

	if pressEnter {
		_, err = ec.execAndCaptureStrict(ctx, containerID, container.ExecOptions{
			Cmd:          []string{"tmux", "send-keys", "-t", sessionName, "Enter"},
			User:         ContainerUser,
			AttachStdout: true,
			AttachStderr: true,
		})
		if err != nil {
			return fmt.Errorf("sending Enter to tmux session: %w", err)
		}
	}

	return nil
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
//
// Uses the combined docker exec path (discoverAndEnrichWorktrees) which merges
// git discovery and terminal listing into a single exec call, falling back to
// the legacy multi-exec path on failure.
func (ec *EngineClient) listWorktreesWithHint(ctx context.Context, containerID string, isGitRepo, skipEnrich bool) ([]Worktree, error) {
	return ec.listWorktreesCombined(ctx, containerID, isGitRepo, skipEnrich)
}

// listWorktreesWithHintLegacy is the original multi-exec implementation,
// kept as a fallback if the combined exec path fails.
func (ec *EngineClient) listWorktreesWithHintLegacy(ctx context.Context, containerID string, isGitRepo, skipEnrich bool) ([]Worktree, error) {
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

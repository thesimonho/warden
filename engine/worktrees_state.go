package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"strings"

	"github.com/docker/docker/api/types/container"
)

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

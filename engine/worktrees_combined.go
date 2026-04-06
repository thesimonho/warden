package engine

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/docker/docker/api/types/container"
)

// discoverAndEnrichWorktrees runs git worktree discovery, terminal directory
// listing, and state enrichment in a single docker exec call. This replaces
// the previous three separate exec calls (discoverGitWorktrees, listDirEntries
// for mergeTerminalWorktrees, and enrichWorktreeState) with one combined script
// that outputs delimited sections.
//
// When skipEnrich is true, only discovery + terminal listing are run (the
// enrichment section is omitted from the script).
func (ec *EngineClient) discoverAndEnrichWorktrees(
	ctx context.Context,
	containerID string,
	isGitRepo, skipEnrich bool,
) ([]Worktree, error) {
	wsDir := ec.workspaceDir(ctx, containerID)
	termDir := wsDir + terminalsDirSuffix

	// For non-git repos, there's only one worktree ("main"). We still need
	// terminal listing and state enrichment, but no git discovery.
	var worktrees []Worktree
	if !isGitRepo {
		worktrees = []Worktree{{
			ID:        "main",
			ProjectID: containerID,
			Path:      wsDir,
			State:     WorktreeStateStopped,
		}}
		// Still need terminal listing + enrichment for the "main" worktree.
		ec.mergeTerminalWorktrees(ctx, containerID, &worktrees)
		if !skipEnrich {
			ec.enrichWorktreeState(ctx, containerID, worktrees)
		}
		return worktrees, nil
	}

	// Build a combined script with three delimited sections:
	// 1. git worktree list --porcelain
	// 2. ls -1 terminals dir
	// 3. batched state enrichment (if !skipEnrich)
	var scriptParts []string

	// Section 1: git worktree list
	scriptParts = append(scriptParts, fmt.Sprintf(
		`git -C %s -c safe.directory=%s worktree list --porcelain 2>/dev/null || true`,
		wsDir, wsDir,
	))
	scriptParts = append(scriptParts, `echo "---GIT_END---"`)

	// Section 2: terminal directory listing
	scriptParts = append(scriptParts, fmt.Sprintf(
		`ls -1 %s 2>/dev/null || true`, termDir,
	))
	scriptParts = append(scriptParts, `echo "---LS_END---"`)

	cmd := strings.Join(scriptParts, " ; ")

	output, err := ec.execAndCapture(ctx, containerID, container.ExecOptions{
		Cmd:          []string{"sh", "-c", cmd},
		User:         ContainerUser,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("combined worktree discovery: %w", err)
	}

	// Parse section 1: git worktree list
	gitSection, rest := splitSection(output, "---GIT_END---")
	worktrees = parseGitWorktreeList(containerID, gitSection, wsDir)

	// Parse section 2: terminal directory entries
	lsSection, _ := splitSection(rest, "---LS_END---")
	terminalEntries := parseDirEntries(lsSection)

	// Merge terminal worktrees (orphans not yet in git)
	mergeTerminalEntries(containerID, wsDir, ec.worktreesPrefixSuffix(ctx, containerID), terminalEntries, &worktrees)

	// Section 3: enrichment via a separate exec call (if needed)
	// The enrichment script is dynamic — it depends on the discovered
	// worktree list which we only know after parsing sections 1+2.
	if !skipEnrich && len(worktrees) > 0 {
		ec.enrichWorktreeState(ctx, containerID, worktrees)
	}

	return worktrees, nil
}

// splitSection splits output at a delimiter line, returning the content before
// and after the delimiter. Uses a newline-prefixed search to ensure the
// delimiter is matched at a line boundary, not inside path content.
func splitSection(output, delimiter string) (before, after string) {
	// Try line-anchored match first (most common: delimiter is on its own line).
	sep := "\n" + delimiter + "\n"
	if idx := strings.Index(output, sep); idx >= 0 {
		return output[:idx], output[idx+len(sep):]
	}
	// Fallback: delimiter may appear at the very start of output or without
	// a trailing newline (e.g. end of output).
	sep = "\n" + delimiter
	if idx := strings.Index(output, sep); idx >= 0 {
		after = output[idx+len(sep):]
		return output[:idx], strings.TrimPrefix(after, "\n")
	}
	// Final fallback: delimiter at start of output (no preceding newline).
	if strings.HasPrefix(output, delimiter) {
		after = output[len(delimiter):]
		return "", strings.TrimPrefix(after, "\n")
	}
	return output, ""
}

// parseDirEntries parses ls -1 output into a list of non-hidden entry names.
func parseDirEntries(output string) []string {
	output = strings.TrimSpace(output)
	if output == "" {
		return nil
	}
	var entries []string
	for _, name := range strings.Split(output, "\n") {
		name = strings.TrimSpace(name)
		if name == "" || strings.HasPrefix(name, ".") {
			continue
		}
		entries = append(entries, name)
	}
	return entries
}

// mergeTerminalEntries adds terminal-only worktrees (not yet in git) to the
// worktree list. This is the data-only version of mergeTerminalWorktrees that
// works with pre-fetched terminal entries instead of running a docker exec.
func mergeTerminalEntries(containerID, wsDir, prefixSuffix string, entries []string, worktrees *[]Worktree) {
	known := make(map[string]bool, len(*worktrees))
	for _, wt := range *worktrees {
		known[wt.ID] = true
	}

	prefix := wsDir + prefixSuffix
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

// listWorktreesCombined is the optimized implementation of listWorktreesWithHint
// that combines git discovery and terminal listing into a single docker exec.
func (ec *EngineClient) listWorktreesCombined(
	ctx context.Context,
	containerID string,
	isGitRepo, skipEnrich bool,
) ([]Worktree, error) {
	worktrees, err := ec.discoverAndEnrichWorktrees(ctx, containerID, isGitRepo, skipEnrich)
	if err != nil {
		slog.Warn("combined worktree discovery failed, falling back to legacy", "container", containerID, "err", err)
		return ec.listWorktreesWithHintLegacy(ctx, containerID, isGitRepo, skipEnrich)
	}
	return worktrees, nil
}

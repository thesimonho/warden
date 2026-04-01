package agent

import "strings"

// MaxToolInputLength is the maximum length of tool input included in events.
const MaxToolInputLength = 1000

// MaxPromptLength is the maximum length of user prompt text included in events.
// Matches the truncation in container event scripts for consistency.
const MaxPromptLength = 500

// WorktreeIDFromCWD extracts the worktree ID from a container-side working
// directory. Returns "main" for the workspace root or unrecognized paths.
//
// Patterns:
//   - .claude/worktrees/<id>  → <id>  (Claude Code native worktrees)
//   - .warden/worktrees/<id>  → <id>  (Warden-managed worktrees for Codex)
func WorktreeIDFromCWD(cwd, workspaceDir string) string {
	for _, prefix := range []string{"/.claude/worktrees/", "/.warden/worktrees/"} {
		if idx := strings.Index(cwd, prefix); idx >= 0 {
			rest := cwd[idx+len(prefix):]
			if slashIdx := strings.Index(rest, "/"); slashIdx >= 0 {
				rest = rest[:slashIdx]
			}
			if rest != "" {
				return rest
			}
		}
	}
	return "main"
}

// TruncateString caps a string at maxLen runes, appending "…" if truncated.
// Uses rune count to avoid splitting multi-byte UTF-8 characters.
func TruncateString(s string, maxLen int) string {
	// Fast path: if byte length fits, rune count fits too (each rune is ≥1 byte).
	if len(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "…"
}

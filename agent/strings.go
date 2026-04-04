package agent

import (
	"regexp"
	"strings"
)

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
func WorktreeIDFromCWD(cwd string) string {
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

// Tag patterns for Claude Code's ! bash mode and /slash command messages.
// These XML-like tags wrap user input, command output, and internal caveats
// in the JSONL session file.
var (
	// Tags to strip entirely (content discarded) — internal instructions
	// and slash-command metadata that carry no audit value.
	stripTags = regexp.MustCompile(
		`(?s)<(?:local-command-caveat|command-name|command-message|command-args|local-command-stdout)>.*?</(?:local-command-caveat|command-name|command-message|command-args|local-command-stdout)>`,
	)

	// Bash input: the user's actual command.
	bashInputTag = regexp.MustCompile(`<bash-input>(.*?)</bash-input>`)

	// Bash output tags — content kept but tags removed.
	bashOutputTags = regexp.MustCompile(`</?(?:bash-stdout|bash-stderr)>`)
)

// FormatPromptText cleans up raw prompt text from agent session files.
// Claude Code's ! bash mode and /slash commands wrap content in XML-like
// tags that are not useful for audit display. This function:
//   - Strips internal instruction tags entirely (local-command-caveat, command-*)
//   - Formats <bash-input> as "$ command"
//   - Unwraps <bash-stdout>/<bash-stderr> content
//   - Returns empty string for prompts that contain only stripped tags
func FormatPromptText(text string) string {
	// Strip tags whose content is not useful for audit.
	result := stripTags.ReplaceAllString(text, "")

	// Format bash input as shell prompt.
	result = bashInputTag.ReplaceAllString(result, "$ $1")

	// Unwrap stdout/stderr tags, keeping their content.
	result = bashOutputTags.ReplaceAllString(result, "")

	result = strings.TrimSpace(result)
	return result
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

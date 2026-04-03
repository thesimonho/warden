package tui

import (
	"errors"
	"regexp"
	"strings"
)

// Worktree name validation patterns — ported from
// web/src/components/project/new-worktree-dialog.tsx.

var invalidWorktreeChars = regexp.MustCompile(`[\s~^:?*\[\]\\@{}\x00-\x1f\x7f]+`)
var consecutiveDots = regexp.MustCompile(`\.{2,}`)
var consecutiveHyphens = regexp.MustCompile(`-{2,}`)
var leadingDotsHyphens = regexp.MustCompile(`^[.-]+`)

// SanitizeWorktreeName replaces invalid git ref characters with hyphens
// and cleans up the result. Returns a name safe for use as a git branch.
func SanitizeWorktreeName(name string) string {
	name = invalidWorktreeChars.ReplaceAllString(name, "-")
	name = consecutiveDots.ReplaceAllString(name, ".")
	name = consecutiveHyphens.ReplaceAllString(name, "-")
	name = leadingDotsHyphens.ReplaceAllString(name, "")
	return name
}

// ValidateWorktreeName checks whether a string is a valid git worktree name.
// Returns nil if valid, or an error describing the problem. Used as a final
// check after sanitization — most character issues are handled by
// [SanitizeWorktreeName].
func ValidateWorktreeName(name string) error {
	if name == "" {
		return errors.New("worktree name is required")
	}
	if strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".") {
		return errors.New("name cannot end with .lock or a dot")
	}
	return nil
}

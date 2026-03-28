package tui

import (
	"errors"
	"regexp"
	"strings"
)

// Worktree name validation patterns — ported from
// web/src/components/project/new-worktree-dialog.tsx.

var invalidWorktreeChars = regexp.MustCompile(`[~^:?*\[\]\\@{}\x00-\x1f\x7f]`)

// ValidateWorktreeName checks whether a string is a valid git worktree name.
// Returns nil if valid, or an error describing the problem.
func ValidateWorktreeName(name string) error {
	if name == "" {
		return errors.New("worktree name is required")
	}
	if strings.ContainsAny(name, " \t\n\r") {
		return errors.New("name cannot contain spaces")
	}
	if strings.HasPrefix(name, "-") {
		return errors.New("name cannot start with a hyphen")
	}
	if strings.HasPrefix(name, ".") {
		return errors.New("name cannot start with a dot")
	}
	if strings.Contains(name, "..") {
		return errors.New("name cannot contain consecutive dots")
	}
	if invalidWorktreeChars.MatchString(name) {
		return errors.New("name contains invalid characters")
	}
	if strings.HasSuffix(name, ".lock") || strings.HasSuffix(name, ".") {
		return errors.New("name cannot end with .lock or a dot")
	}
	return nil
}

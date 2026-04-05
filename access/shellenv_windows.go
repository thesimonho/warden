//go:build windows

package access

import "os/exec"

// configureShellCmd is a no-op on Windows — spawnShell returns early
// before this is called, and Windows shells don't use POSIX process
// groups.
func configureShellCmd(_ *exec.Cmd) {}

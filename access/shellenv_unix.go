//go:build !windows

package access

import (
	"os/exec"
	"syscall"
)

// configureShellCmd puts the shell in a new session so it has no
// controlling terminal. Without this, an interactive login shell
// calls tcsetpgrp() to steal the terminal foreground, which sends
// SIGTTOU to any other process (e.g. the TUI) that tries to write
// to the terminal concurrently.
func configureShellCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}

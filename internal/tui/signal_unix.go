//go:build !windows

package tui

import (
	"os"
	"os/signal"
	"syscall"
)

// notifyResize registers for SIGWINCH signals on Unix systems.
func notifyResize(ch chan<- os.Signal) {
	signal.Notify(ch, syscall.SIGWINCH)
}

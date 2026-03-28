//go:build !windows

package main

import (
	"os"
	"syscall"
)

// shutdownSignals are the OS signals that trigger graceful shutdown.
// On Unix, both SIGINT (Ctrl+C) and SIGTERM (sent by process managers) are caught.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

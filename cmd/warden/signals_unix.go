//go:build !windows

package main

import (
	"os"
	"syscall"
)

// shutdownSignals are the OS signals that trigger graceful shutdown.
var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}

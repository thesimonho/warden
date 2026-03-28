//go:build windows

package main

import "os"

// shutdownSignals are the OS signals that trigger graceful shutdown.
var shutdownSignals = []os.Signal{os.Interrupt}

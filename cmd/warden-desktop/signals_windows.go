//go:build windows

package main

import "os"

// shutdownSignals are the OS signals that trigger graceful shutdown.
// On Windows, only os.Interrupt (Ctrl+C / GenerateConsoleCtrlEvent) is available.
var shutdownSignals = []os.Signal{os.Interrupt}

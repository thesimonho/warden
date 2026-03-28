//go:build windows

package tui

import "os"

// notifyResize is a no-op on Windows. Windows terminals handle resize
// events through the console API, not signals.
func notifyResize(_ chan<- os.Signal) {}

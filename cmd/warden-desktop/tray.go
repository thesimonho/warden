package main

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
)

// startTray attempts to launch the warden-tray companion process.
// Returns the process (for cleanup) or nil if the tray binary is
// unavailable. The tray is non-critical — if it can't start, the
// desktop server continues without it.
func startTray(serverURL string) *os.Process {
	trayPath := findTrayBinary()
	if trayPath == "" {
		slog.Debug("warden-tray binary not found, running without tray")
		return nil
	}

	cmd := exec.Command(trayPath)
	cmd.Env = append(os.Environ(), "WARDEN_URL="+serverURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		slog.Warn("could not start tray", "err", err)
		return nil
	}

	slog.Debug("started warden-tray", "pid", cmd.Process.Pid)
	return cmd.Process
}

// findTrayBinary searches for warden-tray next to the current
// executable (packaged install), then in $PATH (dev/manual install).
func findTrayBinary() string {
	// Check next to the current executable (same directory).
	if exe, err := os.Executable(); err == nil {
		adjacent := filepath.Join(filepath.Dir(exe), "warden-tray")
		if _, err := exec.LookPath(adjacent); err == nil {
			return adjacent
		}
	}

	// Fall back to $PATH.
	if path, err := exec.LookPath("warden-tray"); err == nil {
		return path
	}

	return ""
}

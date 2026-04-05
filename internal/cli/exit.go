// Package cli provides shared utilities for Warden's cmd/ entry points.
package cli

import (
	"context"
	"fmt"
	"os"
	"time"

	warden "github.com/thesimonho/warden"
)

// printRunningContainersTimeout caps the Docker API call so a slow or
// unresponsive daemon doesn't leave the process hanging after the UI exits.
const printRunningContainersTimeout = 3 * time.Second

// PrintRunningContainers lists containers that will keep running after
// the process exits. Helps users understand that Docker containers are
// independent of the Warden process.
func PrintRunningContainers(w *warden.Warden, binaryName string) {
	ctx, cancel := context.WithTimeout(context.Background(), printRunningContainersTimeout)
	defer cancel()

	projects, err := w.Service.ListProjects(ctx)
	if err != nil {
		return
	}
	var running int
	for _, p := range projects {
		if p.State == "running" {
			running++
		}
	}
	if running > 0 {
		fmt.Fprintf(os.Stderr, "\n  %d container(s) still running in Docker.\n", running)
		fmt.Fprintf(os.Stderr, "  Restart %s to reconnect.\n\n", binaryName)
	}
}

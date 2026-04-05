// Package cli provides shared utilities for Warden's cmd/ entry points.
package cli

import (
	"context"
	"fmt"
	"os"

	warden "github.com/thesimonho/warden"
)

// PrintRunningContainers lists containers that will keep running after
// the process exits. Helps users understand that Docker containers are
// independent of the Warden process.
func PrintRunningContainers(w *warden.Warden, binaryName string) {
	projects, err := w.Service.ListProjects(context.Background())
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

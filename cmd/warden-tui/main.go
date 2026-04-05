// Warden TUI — terminal user interface for the Warden container engine.
//
// This binary embeds the Warden engine and provides a terminal-based
// interface for managing projects, worktrees, and agent sessions.
//
// The TUI code in internal/tui/ is written against a Client interface,
// making it a reference implementation for Go developers building their
// own Warden frontends.
package main

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"

	tea "charm.land/bubbletea/v2"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/internal/tui"
)

func main() {
	w, err := warden.New(warden.Options{})
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to start warden: %v\n", err)
		os.Exit(1)
	}

	// Discard slog output after engine init so logs don't bleed into the
	// TUI. Must be after warden.New() which sets its own default logger.
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))

	client := tui.NewServiceAdapter(w)
	p := tea.NewProgram(tui.NewApp(client))

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		w.Close()
		os.Exit(1)
	}

	printRunningContainers(w)
	w.Close()
}

// printRunningContainers lists containers that will keep running after
// the TUI exits. Helps users understand that Docker containers are
// independent of the Warden process.
func printRunningContainers(w *warden.Warden) {
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
		fmt.Fprintf(os.Stderr, "  Restart warden-tui to reconnect.\n\n")
	}
}

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
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/internal/cli"
	"github.com/thesimonho/warden/internal/tui"
)

// shutdownTimeout is the maximum time the process waits for cleanup
// (printing running containers + closing the engine) before force-exiting.
// Prevents the process from hanging after the TUI has disappeared.
const shutdownTimeout = 5 * time.Second

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

	// Failsafe: force-exit if cleanup blocks (e.g. Docker unresponsive).
	go func() {
		time.Sleep(shutdownTimeout)
		os.Exit(0)
	}()

	cli.PrintRunningContainers(w, "warden-tui")
	w.Close()
}

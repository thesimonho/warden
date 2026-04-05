// Warden headless engine server.
//
// Starts the Warden container engine and exposes the HTTP API on the
// configured address. No browser is launched and no frontend assets
// are served — this binary is for developers integrating Warden into
// their own applications.
//
// Environment variables:
//
//	ADDR — listen address (default "127.0.0.1:8090")
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/internal/server"
	"github.com/thesimonho/warden/internal/terminal"
	"github.com/thesimonho/warden/version"
)

func main() {
	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}

	w, err := warden.New(warden.Options{})
	if err != nil {
		slog.Error("failed to start warden", "err", err)
		os.Exit(1)
	}

	termProxy := terminal.NewProxy(w.Engine.APIClient())
	srv := server.New(addr, w.Service, w.Broker, termProxy)

	settings := w.Service.GetSettings()
	url := formatURL(addr)
	slog.Info(
		"starting warden server",
		"url",
		url,
		"runtime",
		settings.Runtime,
		"version",
		version.Version,
	)
	fmt.Fprintf(os.Stderr, "\n  Warden API → %s\n\n", url)
	if !w.DockerAvailable {
		fmt.Fprintln(os.Stderr, "  ┌─────────────────────────────────────────────────────────┐")
		fmt.Fprintln(os.Stderr, "  │  Docker is not available                                │")
		fmt.Fprintln(os.Stderr, "  │  Container operations are disabled.                     │")
		fmt.Fprintln(os.Stderr, "  │                                                         │")
		fmt.Fprintln(os.Stderr, "  │  Install: https://docs.docker.com/get-docker/           │")
		fmt.Fprintln(os.Stderr, "  │                                                         │")
		fmt.Fprintln(os.Stderr, "  │  The API is still serving — read-only endpoints work.   │")
		fmt.Fprintln(os.Stderr, "  └─────────────────────────────────────────────────────────┘")
		fmt.Fprintln(os.Stderr)
	}

	go version.CheckAndPrint()

	// Start the HTTP server in a goroutine.
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	// Block until SIGTERM/SIGINT or API shutdown request.
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()
	select {
	case <-ctx.Done():
	case <-srv.ShutdownCh():
	}

	// Graceful shutdown.
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	printRunningContainers(w)
	w.Close()
}

// printRunningContainers lists containers that will keep running after
// the server exits. Helps users understand that Docker containers are
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
		fmt.Fprintf(os.Stderr, "  Restart warden to reconnect.\n\n")
	}
}

// formatURL builds the HTTP URL from the listen address.
func formatURL(addr string) string {
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	host = strings.Replace(host, "0.0.0.0", "localhost", 1)
	return fmt.Sprintf("http://%s", host)
}

package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"time"

	wardenserver "github.com/thesimonho/warden/internal/server"
)

const (
	serverReadyTimeout = 10 * time.Second
	serverPollInterval = 50 * time.Millisecond
)

// run starts the server, waits for it to be ready, opens the default browser,
// then blocks until SIGTERM/SIGINT before shutting down gracefully.
// The optional cleanup function is called after the server shuts down.
func run(srv *wardenserver.Server, url string, cleanup ...func()) {
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()

	go func() {
		slog.Info("starting dashboard", "url", url)
		fmt.Fprintf(os.Stderr, "\n  Warden dashboard → %s\n\n", url)
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	if err := waitForServer(url+healthPath, serverReadyTimeout); err != nil {
		slog.Error("server did not become ready", "err", err)
		os.Exit(1)
	}

	if os.Getenv("WARDEN_NO_OPEN") == "" {
		openBrowser(url)
	}

	select {
	case <-ctx.Done():
	case <-srv.ShutdownCh():
	}
	shutdown(srv)
	for _, fn := range cleanup {
		fn()
	}
}

// handleExistingInstance opens the browser to the already-running instance.
func handleExistingInstance(url string) {
	slog.Info("connecting to existing instance", "url", url)
	fmt.Fprintf(os.Stderr, "\n  Warden is already running → %s\n\n", url)
	openBrowser(url)
}

// openBrowser opens the given URL in the system default browser using a
// platform-appropriate command. Failures are logged as warnings since the
// server is still running and the URL is printed to stderr.
func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		slog.Warn("could not open browser", "url", url, "err", err)
	}
}

// waitForServer polls the health endpoint until it returns 200 or the timeout
// elapses.
func waitForServer(healthURL string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	client := &http.Client{Timeout: 1 * time.Second}
	ticker := time.NewTicker(serverPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("server not ready after %s", timeout)
		case <-ticker.C:
			resp, err := client.Get(healthURL) //nolint:noctx
			if err == nil {
				resp.Body.Close() //nolint:errcheck
				if resp.StatusCode == http.StatusOK {
					return nil
				}
			}
		}
	}
}

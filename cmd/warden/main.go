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
	slog.Info("starting warden server", "url", url, "runtime", settings.Runtime, "version", version.Version)
	if !w.DockerAvailable {
		fmt.Fprintf(os.Stderr, "\n  Warden API → %s (Docker unavailable — container operations disabled)\n\n", url)
	} else {
		fmt.Fprintf(os.Stderr, "\n  Warden API → %s\n\n", url)
	}

	go version.CheckAndPrint()

	// Start the HTTP server in a goroutine.
	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server exited", "err", err)
			os.Exit(1)
		}
	}()

	// Block until SIGTERM/SIGINT.
	ctx, stop := signal.NotifyContext(context.Background(), shutdownSignals...)
	defer stop()
	<-ctx.Done()

	// Graceful shutdown.
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	w.Close()
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

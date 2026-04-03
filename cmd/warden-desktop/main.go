package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	warden "github.com/thesimonho/warden"
	"github.com/thesimonho/warden/internal/server"
	"github.com/thesimonho/warden/internal/terminal"
)

// healthPath is the API endpoint used for instance detection and readiness polling.
const healthPath = "/api/v1/health"

func main() {
	configureLogging()

	addr := os.Getenv("ADDR")
	if addr == "" {
		addr = "127.0.0.1:8090"
	}

	url := browserURL(addr)

	// If another instance is already running, hand off to it.
	if isAlreadyRunning(url) {
		handleExistingInstance(url)
		return
	}

	w, err := warden.New(warden.Options{})
	if err != nil {
		slog.Error("failed to start warden", "err", err)
		os.Exit(1)
	}
	settings := w.Service.GetSettings()
	slog.Info("container runtime", "runtime", settings.Runtime)

	termProxy := terminal.NewProxy(w.Engine.APIClient())
	srv := server.New(addr, w.Service, w.Broker, termProxy)

	run(srv, url, func() {
		w.Close()
	})
}

// isAlreadyRunning checks if a Warden instance is already listening by hitting
// the health endpoint and verifying the X-Warden header.
func isAlreadyRunning(url string) bool {
	client := &http.Client{Timeout: 500 * time.Millisecond}
	resp, err := client.Get(url + healthPath) //nolint:noctx
	if err != nil {
		return false
	}
	resp.Body.Close() //nolint:errcheck
	return resp.StatusCode == http.StatusOK && resp.Header.Get("X-Warden") == "1"
}

// configureLogging sets the default slog level from WARDEN_LOG_LEVEL
// (debug, info, warn, error). Defaults to info when unset.
func configureLogging() {
	levelStr := os.Getenv("WARDEN_LOG_LEVEL")
	if levelStr == "" {
		return
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(levelStr)); err != nil {
		slog.Warn("invalid WARDEN_LOG_LEVEL, using default", "value", levelStr)
		return
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))
}

// browserURL builds the HTTP URL from the listen address, replacing
// unroutable bind addresses with localhost for browser/webview navigation.
func browserURL(addr string) string {
	host := addr
	if strings.HasPrefix(host, ":") {
		host = "localhost" + host
	}
	host = strings.Replace(host, "0.0.0.0", "localhost", 1)
	return fmt.Sprintf("http://%s", host)
}

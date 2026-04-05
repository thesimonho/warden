// Warden Tray — system tray companion for warden-desktop.
//
// Provides a persistent tray icon so users know Warden is running
// after closing the browser. Communicates with the Warden server
// over HTTP — no warden packages are imported, keeping CGo
// isolated to this binary.
//
// Environment:
//
//	WARDEN_URL — server base URL (default "http://127.0.0.1:8090")
package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	baseURL := os.Getenv("WARDEN_URL")
	if baseURL == "" {
		baseURL = "http://127.0.0.1:8090"
	}

	srv := newServerClient(baseURL)

	// Wait for the server to be reachable before showing the tray.
	if !srv.waitForReady() {
		log.Fatal("warden server did not become ready")
	}

	// Handle OS signals for clean exit.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigCh
		quitTray()
	}()

	runTray(srv)
}

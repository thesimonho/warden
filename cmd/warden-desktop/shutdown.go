package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/thesimonho/warden/internal/server"
)

const shutdownTimeout = 10 * time.Second

// shutdown gracefully stops the HTTP server within shutdownTimeout.
func shutdown(srv *server.Server) {
	slog.Info("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("shutdown error", "err", err)
		os.Exit(1)
	}
}

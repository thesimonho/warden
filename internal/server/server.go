package server

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"time"

	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/internal/terminal"
	"github.com/thesimonho/warden/service"
	"github.com/thesimonho/warden/version"
)

//go:embed all:ui
var staticFiles embed.FS

// Server holds the configured HTTP server.
type Server struct {
	addr       string
	httpServer *http.Server
}

// New creates a Server listening on addr with the given service, SSE event
// broker, and terminal proxy wired into all API route handlers.
func New(addr string, svc *service.Service, broker *eventbus.Broker, termProxy *terminal.Proxy) *Server {
	mux := http.NewServeMux()

	// API routes
	registerAPIRoutes(mux, svc, broker, termProxy)

	// Static frontend — served from embedded ui/ directory
	uiFS, err := fs.Sub(staticFiles, "ui")
	if err != nil {
		panic("failed to sub ui embed: " + err.Error())
	}
	mux.Handle("/", spaHandler(http.FileServerFS(uiFS), uiFS))

	handler := loggingMiddleware(corsMiddleware(mux))

	return &Server{
		addr: addr,
		httpServer: &http.Server{
			Addr:              addr,
			Handler:           handler,
			ReadHeaderTimeout: 10 * time.Second,
			IdleTimeout:       120 * time.Second,
		},
	}
}

// Start begins listening and serving. Blocks until the server exits.
func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

// Shutdown gracefully shuts down the server without interrupting active connections.
func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

/**
 * spaHandler wraps a file server with SPA fallback behavior.
 *
 * If the requested path matches a real file in the embedded FS, it's
 * served directly. Otherwise, index.html is served so that the React
 * client-side router can handle the route.
 */
func spaHandler(fileServer http.Handler, fsys fs.FS) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "/" {
			fileServer.ServeHTTP(w, r)
			return
		}

		// Strip leading slash for fs.Stat lookup.
		filePath := path[1:]
		if _, err := fs.Stat(fsys, filePath); err == nil {
			fileServer.ServeHTTP(w, r)
			return
		}

		// File not found — serve index.html for client-side routing.
		r.URL.Path = "/"
		fileServer.ServeHTTP(w, r)
	})
}

// handleHealth returns a simple health check response.
//
//	@Summary		Health check
//	@Description	Returns a simple health check response indicating the server is running.
//	@Tags			health
//	@Success		200	{object}	healthResponse
//	@Router			/api/v1/health [get]
func handleHealth(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("X-Warden", "1")
	writeJSON(w, healthResponse{Status: "ok", Version: version.Version})
}

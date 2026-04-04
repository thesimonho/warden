package server

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// --- SSE & WebSocket ---

// handleSSE streams Server-Sent Events to the client.
//
//	@Summary		Subscribe to events (SSE)
//	@Description	Opens a Server-Sent Events stream for real-time updates. Event types:
//	@Description	worktree_state (attention/terminal changes), project_state (cost + attention updates),
//	@Description	worktree_list_changed (worktree added/removed), heartbeat (keepalive every 15s).
//	@Tags			streaming
//	@Success		200	{string}	string	"SSE event stream"
//	@Failure		500	{object}	apiError
//	@Failure		503	{object}	apiError	"Event streaming not configured"
//	@Router			/api/v1/events [get]
func (rt *routes) handleSSE(w http.ResponseWriter, r *http.Request) {
	if rt.broker == nil {
		writeError(w, ErrCodeNotConfigured, "event streaming not configured", http.StatusServiceUnavailable)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, ErrCodeInternal, "streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")
	flusher.Flush()

	ch, unsubscribe := rt.broker.Subscribe()
	defer unsubscribe()

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event.Event, event.Data) //nolint:errcheck
			flusher.Flush()
		}
	}
}

// clipboardMaxSize is the maximum upload size for clipboard images (10 MB).
const clipboardMaxSize = 10 << 20

// handleUploadClipboard stages an image in the container's clipboard directory
// for the xclip shim to serve. The web frontend calls this before sending Ctrl+V
// to the terminal so the agent's clipboard read picks up the image.
//
//	@Summary		Upload clipboard image
//	@Description	Stages an image file in the container's clipboard directory. The xclip
//	@Description	shim serves it when the agent reads the clipboard. Used by the web
//	@Description	frontend for image paste support.
//	@Tags			clipboard
//	@Accept			multipart/form-data
//	@Produce		json
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			file		formData	file							true	"Image file"
//	@Success		200			{object}	api.ClipboardUploadResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		413			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/clipboard [post]
func (rt *routes) handleUploadClipboard(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, clipboardMaxSize)

	if err := r.ParseMultipartForm(clipboardMaxSize); err != nil {
		writeError(w, ErrCodeInvalidBody, "file too large or invalid multipart form", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, ErrCodeRequiredField, "file field is required", http.StatusBadRequest)
		return
	}
	defer func() { _ = file.Close() }()

	content, err := io.ReadAll(file)
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to read file", http.StatusInternalServerError)
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "image/png"
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.UploadClipboard(r.Context(), projectID, agentType, content, mimeType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("upload clipboard", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, resp)
}

// handleTerminalWS upgrades to a WebSocket and bridges it to a tmux session
// inside the project's container via docker exec.
//
//	@Summary		Terminal WebSocket
//	@Description	Upgrades to a WebSocket connection and bridges it to a tmux terminal session
//	@Description	inside the container via docker exec. Binary frames carry raw PTY data;
//	@Description	text frames carry JSON control messages (e.g. {"type":"resize","cols":80,"rows":24}).
//	@Tags			streaming
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			wid			path	string	true	"Worktree ID"
//	@Success		101			"WebSocket upgrade"
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		503			{object}	apiError	"Terminal proxy not configured"
//	@Router			/api/v1/projects/{projectId}/{agentType}/ws/{wid} [get]
func (rt *routes) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	// The WebSocket handler needs the container ID for docker exec and the
	// full project row for NotifyTerminalDisconnected, so resolve via GetProject.
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}
	row, err := rt.svc.GetProject(projectID, agentType)
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to look up project", http.StatusInternalServerError)
		slog.Error("resolve project for terminal WS", "projectId", projectID, "err", err)
		return
	}
	if row == nil {
		writeError(w, ErrCodeNotFound, "project not found", http.StatusNotFound)
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	if rt.terminalProxy == nil {
		writeError(w, ErrCodeNotConfigured, "terminal proxy not configured", http.StatusServiceUnavailable)
		return
	}

	// Track active viewer count so that closing a stale WebSocket (e.g.
	// from React Strict Mode double-mount) doesn't mark the terminal as
	// disconnected while a newer connection is still open.
	vk := viewerKey{containerID: row.ContainerID, worktreeID: wid}
	rt.viewerMu.Lock()
	rt.viewerCounts[vk]++
	rt.viewerMu.Unlock()

	// ServeWS blocks until the WebSocket closes.
	rt.terminalProxy.ServeWS(w, r, row.ContainerID, wid)

	// Only push terminal_disconnected when the last viewer closes.
	rt.viewerMu.Lock()
	rt.viewerCounts[vk]--
	remaining := rt.viewerCounts[vk]
	if remaining <= 0 {
		delete(rt.viewerCounts, vk)
	}
	rt.viewerMu.Unlock()

	if remaining <= 0 {
		// Use a fresh context — the request context may be cancelled
		// after the WebSocket handler returns.
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		rt.svc.NotifyTerminalDisconnected(ctx, row, wid)
	}
}

package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/internal/terminal"
	"github.com/thesimonho/warden/service"
)

// containerIDRegex validates Docker short (12-char) or full (64-char) hex container IDs.
var containerIDRegex = regexp.MustCompile(`^[a-f0-9]{12,64}$`)

// domainRegex validates domain names (e.g. "api.anthropic.com", "*.github.com").
var domainRegex = regexp.MustCompile(`^(\*\.)?[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]*[a-zA-Z0-9])?)*$`)

// containerNameRegex validates Docker container names (alphanumeric, hyphens, underscores, dots).
var containerNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.\-]*$`)

// viewerKey identifies a worktree's WebSocket viewer connections.
type viewerKey struct {
	containerID string
	worktreeID  string
}

// routes holds dependencies and provides HTTP handler methods.
type routes struct {
	svc           *service.Service
	broker        *eventbus.Broker
	terminalProxy *terminal.Proxy

	// viewerMu guards viewerCounts for concurrent WebSocket connect/disconnect.
	viewerMu     sync.Mutex
	viewerCounts map[viewerKey]int
}

// registerAPIRoutes attaches all API endpoint handlers to the given mux.
func registerAPIRoutes(mux *http.ServeMux, svc *service.Service, broker *eventbus.Broker, termProxy *terminal.Proxy) {
	rt := &routes{svc: svc, broker: broker, terminalProxy: termProxy, viewerCounts: make(map[viewerKey]int)}

	mux.HandleFunc("GET /api/v1/health", handleHealth)
	mux.HandleFunc("GET /api/v1/projects", rt.handleListProjects)
	mux.HandleFunc("POST /api/v1/projects", rt.handleAddProject)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}", rt.handleRemoveProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/stop", rt.handleStopProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/restart", rt.handleRestartProject)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/worktrees", rt.handleListWorktrees)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/worktrees", rt.handleCreateWorktree)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/worktrees/{wid}/connect", rt.handleConnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/worktrees/{wid}/disconnect", rt.handleDisconnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/worktrees/{wid}/kill", rt.handleKillWorktreeProcess)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/worktrees/{wid}", rt.handleRemoveWorktree)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/worktrees/{wid}/diff", rt.handleGetWorktreeDiff)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/worktrees/cleanup", rt.handleCleanupWorktrees)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/costs", rt.handleResetProjectCosts)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/audit", rt.handlePurgeProjectAudit)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/container", rt.handleCreateContainer)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/container", rt.handleDeleteContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/container/config", rt.handleInspectContainer)
	mux.HandleFunc("PUT /api/v1/projects/{projectId}/container", rt.handleUpdateContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/container/validate", rt.handleValidateContainer)
	mux.HandleFunc("GET /api/v1/runtimes", rt.handleListRuntimes)
	mux.HandleFunc("GET /api/v1/settings", rt.handleGetSettings)
	mux.HandleFunc("PUT /api/v1/settings", rt.handleUpdateSettings)
	mux.HandleFunc("GET /api/v1/audit", rt.handleGetAuditLog)
	mux.HandleFunc("GET /api/v1/audit/summary", rt.handleGetAuditSummary)
	mux.HandleFunc("GET /api/v1/audit/export", rt.handleExportAuditLog)
	mux.HandleFunc("GET /api/v1/audit/projects", rt.handleGetAuditProjects)
	mux.HandleFunc("POST /api/v1/audit", rt.handlePostAuditEvent)
	mux.HandleFunc("DELETE /api/v1/audit", rt.handleDeleteAuditEvents)
	mux.HandleFunc("GET /api/v1/filesystem/directories", rt.handleListDirectories)
	mux.HandleFunc("POST /api/v1/filesystem/reveal", rt.handleRevealInFileManager)
	mux.HandleFunc("GET /api/v1/access", rt.handleListAccessItems)
	mux.HandleFunc("POST /api/v1/access", rt.handleCreateAccessItem)
	mux.HandleFunc("GET /api/v1/access/{id}", rt.handleGetAccessItem)
	mux.HandleFunc("PUT /api/v1/access/{id}", rt.handleUpdateAccessItem)
	mux.HandleFunc("DELETE /api/v1/access/{id}", rt.handleDeleteAccessItem)
	mux.HandleFunc("POST /api/v1/access/{id}/reset", rt.handleResetAccessItem)
	mux.HandleFunc("POST /api/v1/access/resolve", rt.handleResolveAccessItems)
	mux.HandleFunc("GET /api/v1/defaults", rt.handleDefaults)
	mux.HandleFunc("GET /api/v1/events", rt.handleSSE)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/ws/{wid}", rt.handleTerminalWS)
}

// --- Helpers ---

// resolveProject looks up the project row from a projectId path parameter.
// Writes a 404 error response and returns nil if the project is not found.
func (rt *routes) resolveProject(w http.ResponseWriter, r *http.Request) *db.ProjectRow {
	projectID := r.PathValue("projectId")
	if !engine.ValidProjectID(projectID) {
		writeError(w, ErrCodeInvalidBody, "invalid project ID", http.StatusBadRequest)
		return nil
	}
	row, err := rt.svc.GetProject(projectID)
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to look up project", http.StatusInternalServerError)
		slog.Error("resolve project", "projectId", projectID, "err", err)
		return nil
	}
	if row == nil {
		writeError(w, ErrCodeNotFound, "project not found", http.StatusNotFound)
		return nil
	}
	return row
}

// --- Projects ---

// handleListProjects returns all projects from the config, enriched with Docker state.
//
//	@Summary		List projects
//	@Description	Returns all configured projects enriched with live container state,
//	@Description	Claude status, worktree counts, and cost data.
//	@Tags			projects
//	@Success		200	{array}		engine.Project
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/projects [get]
func (rt *routes) handleListProjects(w http.ResponseWriter, r *http.Request) {
	projects, err := rt.svc.ListProjects(r.Context())
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("list projects", "err", err)
		return
	}
	writeJSON(w, projects)
}

// handleAddProject registers a project directory in Warden.
//
//	@Summary		Add project
//	@Description	Registers a host directory as a Warden project.
//	@Tags			projects
//	@Accept			json
//	@Param			body	body		addProjectRequest	true	"Project details"
//	@Success		201		{object}	service.ProjectResult
//	@Failure		400		{object}	apiError
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/projects [post]
func (rt *routes) handleAddProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req struct {
		Name        string `json:"name"`
		ProjectPath string `json:"projectPath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" {
		writeError(w, ErrCodeInvalidBody, "projectPath is required", http.StatusBadRequest)
		return
	}

	if req.Name != "" && !isValidContainerName(req.Name) {
		writeError(w, ErrCodeInvalidContainerName, "invalid container name", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.AddProject(req.Name, req.ProjectPath)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("add project", "name", req.Name, "err", err)
		return
	}

	writeJSONCreated(w, result)
}

// handleRemoveProject removes a project from the database by project ID.
//
//	@Summary		Remove project
//	@Description	Removes a project by its ID. Does not stop or delete the container.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	service.ProjectResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId} [delete]
func (rt *routes) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	result, err := rt.svc.RemoveProject(row.ProjectID)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("remove project", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleResetProjectCosts removes all cost history for a project.
//
//	@Summary		Reset project costs
//	@Description	Removes all cost history for the given project.
//	@Tags			projects
//	@Param			projectId	path	string	true	"Project ID"
//	@Success		204
//	@Failure		400	{object}	apiError
//	@Failure		404	{object}	apiError
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/projects/{projectId}/costs [delete]
func (rt *routes) handleResetProjectCosts(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	if err := rt.svc.ResetProjectCosts(row.ProjectID); err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("reset project costs", "projectId", row.ProjectID, "err", err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePurgeProjectAudit removes all audit events for a project.
//
//	@Summary		Purge project audit
//	@Description	Removes all audit events for the given project.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	map[string]int64
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/audit [delete]
func (rt *routes) handlePurgeProjectAudit(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	deleted, err := rt.svc.PurgeProjectAudit(row.ProjectID)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("purge project audit", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, map[string]int64{"deleted": deleted})
}

// handleStopProject stops the container for the given project.
//
//	@Summary		Stop project
//	@Description	Gracefully stops the container for the given project.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	service.ProjectResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/stop [post]
func (rt *routes) handleStopProject(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	result, err := rt.svc.StopProject(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("stop project", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleRestartProject restarts the container for the given project.
//
//	@Summary		Restart project
//	@Description	Restarts the container for the given project. Fails with STALE_MOUNTS if bind mounts reference missing host paths.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	service.ProjectResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		409			{object}	apiError	"Stale mounts prevent restart"
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/restart [post]
func (rt *routes) handleRestartProject(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	result, err := rt.svc.RestartProject(r.Context(), row)
	if err != nil {
		if errors.Is(err, service.ErrBudgetExceeded) {
			writeError(w, ErrCodeBudgetExceeded, err.Error(), http.StatusForbidden)
			return
		}
		if engine.IsStaleMountsError(err) {
			writeError(w, ErrCodeStaleMounts, err.Error(), http.StatusConflict)
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("restart project", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// --- Worktrees ---

// handleListWorktrees returns all worktrees for the given project with their terminal state.
//
//	@Summary		List worktrees
//	@Description	Returns all worktrees for the given project, including terminal connection state,
//	@Description	Claude attention status, and git branch information.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{array}		engine.Worktree
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees [get]
func (rt *routes) handleListWorktrees(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	worktrees, err := rt.svc.ListWorktrees(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("list worktrees", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, worktrees)
}

// handleCreateWorktree creates a new git worktree and connects a terminal.
//
//	@Summary		Create worktree
//	@Description	Creates a new git worktree inside the container and automatically connects a terminal.
//	@Tags			worktrees
//	@Accept			json
//	@Param			projectId	path		string					true	"Project ID"
//	@Param			body		body		createWorktreeRequest	true	"Worktree name (must be a valid git branch name)"
//	@Success		201			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees [post]
func (rt *routes) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		writeError(w, ErrCodeRequiredField, "worktree name is required", http.StatusBadRequest)
		return
	}

	if !isValidWorktreeID(req.Name) {
		writeError(w, ErrCodeInvalidWorktreeName, "invalid worktree name", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.CreateWorktree(r.Context(), row, req.Name)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("create worktree", "projectId", row.ProjectID, "name", req.Name, "err", err)
		return
	}

	writeJSONCreated(w, resp)
}

// handleConnectTerminal starts a terminal for a worktree in the given project.
//
//	@Summary		Connect terminal
//	@Description	Starts an abduco terminal session for the given worktree. If a background session
//	@Description	already exists, reconnects to it instead of creating a new one.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		201			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/{wid}/connect [post]
func (rt *routes) handleConnectTerminal(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.ConnectTerminal(r.Context(), row, wid)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("connect terminal", "projectId", row.ProjectID, "wid", wid, "err", err)
		return
	}

	writeJSONCreated(w, resp)
}

// handleDisconnectTerminal kills the terminal viewer for a worktree.
//
//	@Summary		Disconnect terminal
//	@Description	Closes the terminal viewer WebSocket. The abduco session (and Claude/bash)
//	@Description	continues running in the background.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/{wid}/disconnect [post]
func (rt *routes) handleDisconnectTerminal(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.DisconnectTerminal(r.Context(), row, wid)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("disconnect terminal", "projectId", row.ProjectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleKillWorktreeProcess kills abduco and all child processes for a worktree.
//
//	@Summary		Kill worktree process
//	@Description	Kills the abduco session and all child processes for the worktree.
//	@Description	The git worktree directory on disk is preserved. This is destructive —
//	@Description	any running Claude session is terminated immediately.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/{wid}/kill [post]
func (rt *routes) handleKillWorktreeProcess(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.KillWorktreeProcess(r.Context(), row, wid)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("kill worktree process", "projectId", row.ProjectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleRemoveWorktree fully removes a worktree: kills processes, runs
// `git worktree remove`, and cleans up tracking state.
//
//	@Summary		Remove worktree
//	@Description	Fully removes a worktree: kills any running processes, runs `git worktree remove`,
//	@Description	and cleans up tracking state. Cannot remove the "main" worktree.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/{wid} [delete]
func (rt *routes) handleRemoveWorktree(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.RemoveWorktree(r.Context(), row, wid)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("remove worktree", "projectId", row.ProjectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleCleanupWorktrees removes orphaned worktree directories.
//
//	@Summary		Cleanup orphaned worktrees
//	@Description	Removes worktree directories that are not tracked by git, kills orphaned abduco
//	@Description	sessions, and prunes stale git worktree metadata.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	cleanupWorktreesResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/cleanup [post]
func (rt *routes) handleCleanupWorktrees(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	removed, err := rt.svc.CleanupWorktrees(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("cleanup worktrees", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, cleanupWorktreesResponse{Removed: removed})
}

// handleGetWorktreeDiff returns uncommitted changes for a worktree.
//
//	@Summary		Get worktree diff
//	@Description	Returns uncommitted changes (tracked and untracked files) for a worktree
//	@Description	as a unified diff with per-file statistics.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	api.DiffResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/worktrees/{wid}/diff [get]
func (rt *routes) handleGetWorktreeDiff(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.GetWorktreeDiff(r.Context(), row, wid)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get worktree diff", "projectId", row.ProjectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, resp)
}

// --- Containers ---

// handleCreateContainer creates a new container for an existing project.
//
//	@Summary		Create container
//	@Description	Creates a new container for the given project with the provided configuration.
//	@Description	Supports network isolation modes and custom bind mounts.
//	@Tags			containers
//	@Accept			json
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			body		body		engine.CreateContainerRequest	true	"Container configuration"
//	@Success		201			{object}	service.ContainerResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		409			{object}	apiError	"Container name already in use"
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/container [post]
func (rt *routes) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req engine.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if !isValidContainerName(req.Name) {
		writeError(w, ErrCodeInvalidContainerName, "invalid container name", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" {
		writeError(w, ErrCodeRequiredField, "project path is required", http.StatusBadRequest)
		return
	}

	if msg := validateNetworkConfig(req); msg != "" {
		writeError(w, ErrCodeInvalidNetworkConfig, msg, http.StatusBadRequest)
		return
	}

	result, err := rt.svc.CreateContainer(r.Context(), req)
	if err != nil {
		slog.Error("create container", "projectId", row.ProjectID, "name", req.Name, "err", err)
		if errors.Is(err, engine.ErrNameTaken) {
			writeError(w, ErrCodeNameTaken, err.Error(), http.StatusConflict)
		} else {
			writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	writeJSONCreated(w, result)
}

// handleDeleteContainer stops and removes the container for the given project.
//
//	@Summary		Delete container
//	@Description	Stops and removes the container for the given project. The container is permanently deleted.
//	@Tags			containers
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	service.ContainerResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/container [delete]
func (rt *routes) handleDeleteContainer(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	result, err := rt.svc.DeleteContainer(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("delete container", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleInspectContainer returns the editable configuration of the project's container.
//
//	@Summary		Get container config
//	@Description	Returns the editable configuration of the project's container, including name, image,
//	@Description	project path, bind mounts, environment variables, and network settings.
//	@Tags			containers
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	engine.ContainerConfig
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/container/config [get]
func (rt *routes) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	cfg, err := rt.svc.InspectContainer(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("inspect container", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, cfg)
}

// handleValidateContainer checks whether the project's container has Warden terminal infrastructure.
//
//	@Summary		Validate container infrastructure
//	@Description	Checks whether the project's running container has the required Warden terminal
//	@Description	infrastructure installed (abduco, create-terminal.sh).
//	@Tags			containers
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	validateContainerResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/container/validate [get]
func (rt *routes) handleValidateContainer(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	result, err := rt.svc.ValidateContainer(r.Context(), row)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("validate infrastructure", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, validateContainerResponse{Valid: result.Valid, Missing: result.Missing})
}

// handleUpdateContainer recreates the project's container with updated configuration.
//
//	@Summary		Update container
//	@Description	Recreates the project's container with updated configuration. The old container is stopped
//	@Description	and removed, then a new one is created with the provided settings.
//	@Tags			containers
//	@Accept			json
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			body		body		engine.CreateContainerRequest	true	"Updated container configuration"
//	@Success		200			{object}	service.ContainerResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/container [put]
func (rt *routes) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req engine.CreateContainerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" {
		writeError(w, ErrCodeRequiredField, "project path is required", http.StatusBadRequest)
		return
	}

	if msg := validateNetworkConfig(req); msg != "" {
		writeError(w, ErrCodeInvalidNetworkConfig, msg, http.StatusBadRequest)
		return
	}

	result, err := rt.svc.UpdateContainer(r.Context(), row, req)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("update container", "projectId", row.ProjectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// --- Settings & Audit Log ---

// handleGetSettings returns the server-side settings.
//
//	@Summary		Get settings
//	@Description	Returns the current server-side settings including runtime, audit log state, and disconnect key.
//	@Tags			settings
//	@Success		200	{object}	service.SettingsResponse
//	@Router			/api/v1/settings [get]
func (rt *routes) handleGetSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, rt.svc.GetSettings())
}

// handleUpdateSettings updates server-side settings.
//
//	@Summary		Update settings
//	@Description	Updates server-side settings. Only provided fields are changed. Changing the
//	@Description	runtime requires a server restart. Changing auditLogMode syncs the flag to all running containers.
//	@Tags			settings
//	@Accept			json
//	@Param			body	body		service.UpdateSettingsRequest	true	"Settings to update (all fields optional)"
//	@Success		200		{object}	updateSettingsResponse
//	@Failure		400		{object}	apiError
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/settings [put]
func (rt *routes) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req service.UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.UpdateSettings(r.Context(), req)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, updateSettingsResponse{RestartRequired: result.RestartRequired})
}

// --- Audit Log ---

// handleGetAuditLog returns filtered audit events.
//
//	@Summary		Get audit log
//	@Description	Returns audit-relevant events (sessions, tool uses, prompts, lifecycle)
//	@Description	with optional filtering by project, worktree, category, and time range.
//	@Tags			audit
//	@Param			projectId	query		string	false	"Filter by project ID"
//	@Param			worktree	query		string	false	"Filter by worktree ID"
//	@Param			category	query		string	false	"Filter by category (session, agent, prompt, config, budget, system, debug)"
//	@Param			source		query		string	false	"Filter by source (agent, backend, frontend, container)"
//	@Param			level		query		string	false	"Filter by level (info, warning, error)"
//	@Param			since		query		string	false	"Filter entries after this timestamp (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this timestamp (RFC3339)"
//	@Param			limit		query		int		false	"Maximum entries to return (default 10000)"
//	@Param			offset		query		int		false	"Number of entries to skip"
//	@Success		200			{array}		db.Entry
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/audit [get]
func (rt *routes) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	offset, _ := strconv.Atoi(q.Get("offset"))

	filters := service.AuditFilters{
		ProjectID: q.Get("projectId"),
		Worktree:  q.Get("worktree"),
		Category:  service.AuditCategory(q.Get("category")),
		Source:    q.Get("source"),
		Level:     q.Get("level"),
		Since:     q.Get("since"),
		Until:     q.Get("until"),
		Limit:     limit,
		Offset:    offset,
	}

	entries, err := rt.svc.GetAuditLog(filters)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("read audit log", "err", err)
		return
	}

	if entries == nil {
		entries = []db.Entry{}
	}
	writeJSON(w, entries)
}

// handleGetAuditSummary returns aggregate audit statistics.
//
//	@Summary		Get audit summary
//	@Description	Returns aggregate statistics for audit events including session counts,
//	@Description	tool usage, cost totals, and top tools.
//	@Tags			audit
//	@Param			projectId	query		string	false	"Filter by project ID"
//	@Param			worktree	query		string	false	"Filter by worktree ID"
//	@Param			since		query		string	false	"Filter entries after this timestamp (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this timestamp (RFC3339)"
//	@Success		200			{object}	api.AuditSummary
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/audit/summary [get]
func (rt *routes) handleGetAuditSummary(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := service.AuditFilters{
		ProjectID: q.Get("projectId"),
		Worktree:  q.Get("worktree"),
		Since:     q.Get("since"),
		Until:     q.Get("until"),
	}

	summary, err := rt.svc.GetAuditSummary(r.Context(), filters)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get audit summary", "err", err)
		return
	}

	writeJSON(w, summary)
}

// handleExportAuditLog exports audit events as CSV or JSON.
//
//	@Summary		Export audit log
//	@Description	Downloads audit events as CSV or JSON for compliance review.
//	@Tags			audit
//	@Param			format		query		string	false	"Export format: csv or json (default json)"
//	@Param			projectId	query		string	false	"Filter by project ID"
//	@Param			worktree	query		string	false	"Filter by worktree ID"
//	@Param			category	query		string	false	"Filter by category (session, agent, prompt, config, budget, system, debug)"
//	@Param			since		query		string	false	"Filter entries after this timestamp (RFC3339)"
//	@Param			until		query		string	false	"Filter entries before this timestamp (RFC3339)"
//	@Success		200			{file}		file
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/audit/export [get]
func (rt *routes) handleExportAuditLog(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	format := q.Get("format")
	if format == "" {
		format = "json"
	}

	filters := service.AuditFilters{
		ProjectID: q.Get("projectId"),
		Worktree:  q.Get("worktree"),
		Category:  service.AuditCategory(q.Get("category")),
		Since:     q.Get("since"),
		Until:     q.Get("until"),
	}

	timestamp := time.Now().UTC().Format("2006-01-02T150405")

	switch format {
	case "csv":
		w.Header().Set("Content-Type", "text/csv")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="warden-audit-%s.csv"`, timestamp))
		if err := rt.svc.WriteAuditCSV(w, filters); err != nil {
			slog.Error("export audit CSV", "err", err)
		}

	default:
		w.Header().Set("Content-Type", "application/x-ndjson")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="warden-audit-%s.jsonl"`, timestamp))

		entries, err := rt.svc.GetAuditLog(filters)
		if err != nil {
			writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
			slog.Error("export audit JSON", "err", err)
			return
		}

		enc := json.NewEncoder(w)
		for _, entry := range entries {
			if encErr := enc.Encode(entry); encErr != nil {
				slog.Error("encode audit entry", "err", encErr)
				return
			}
		}
	}
}

// handleGetAuditProjects returns distinct project names from the audit log.
//
//	@Summary		List audit projects
//	@Description	Returns distinct project IDs that have audit events recorded.
//	@Tags			audit
//	@Success		200	{array}		string
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/audit/projects [get]
func (rt *routes) handleGetAuditProjects(w http.ResponseWriter, _ *http.Request) {
	projects, err := rt.svc.GetAuditProjects()
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get audit projects", "err", err)
		return
	}

	if projects == nil {
		projects = []string{}
	}
	writeJSON(w, projects)
}

// handlePostAuditEvent accepts a frontend event and writes it to the audit log.
//
//	@Summary		Post audit event
//	@Description	Writes a custom audit event from the frontend or an external consumer.
//	@Tags			audit
//	@Accept			json
//	@Param			body	body	api.PostAuditEventRequest	true	"Audit event to record"
//	@Success		204
//	@Failure		400	{object}	apiError
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/audit [post]
func (rt *routes) handlePostAuditEvent(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4<<10)

	var req service.PostAuditEventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.Event == "" {
		writeError(w, ErrCodeRequiredField, "event is required", http.StatusBadRequest)
		return
	}

	if err := rt.svc.PostAuditEvent(req); err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleDeleteAuditEvents removes events matching query filters.
//
//	@Summary		Delete audit events
//	@Description	Removes audit events matching the given filters. Supports scoping by
//	@Description	project, worktree, and time range.
//	@Tags			audit
//	@Param			projectId	query		string	false	"Filter by project ID"
//	@Param			worktree	query		string	false	"Filter by worktree ID"
//	@Param			since		query		string	false	"Delete entries after this timestamp (RFC3339)"
//	@Param			until		query		string	false	"Delete entries before this timestamp (RFC3339)"
//	@Success		200			{object}	object{deleted=int64}
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/audit [delete]
func (rt *routes) handleDeleteAuditEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()

	filters := service.AuditFilters{
		ProjectID: q.Get("projectId"),
		Worktree:  q.Get("worktree"),
		Since:     q.Get("since"),
		Until:     q.Get("until"),
	}

	deleted, err := rt.svc.DeleteAuditEvents(filters)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("delete audit events", "err", err)
		return
	}

	writeJSON(w, map[string]int64{"deleted": deleted})
}

// --- Host Utilities ---

// handleDefaults returns server-resolved default values for the create container form.
//
//	@Summary		Get defaults
//	@Description	Returns server-resolved default values for the create container form,
//	@Description	including the host home directory and auto-detected bind mounts.
//	@Tags			host
//	@Success		200	{object}	service.DefaultsResponse
//	@Router			/api/v1/defaults [get]
// --- Access item handlers ---

func (rt *routes) handleListAccessItems(w http.ResponseWriter, _ *http.Request) {
	items, err := rt.svc.ListAccessItems()
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to list access items", http.StatusInternalServerError)
		slog.Error("list access items", "err", err)
		return
	}
	writeJSON(w, api.AccessItemListResponse{Items: items})
}

func (rt *routes) handleGetAccessItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := rt.svc.GetAccessItem(id)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, ErrCodeNotFound, "access item not found", http.StatusNotFound)
			return
		}
		writeError(w, ErrCodeInternal, "failed to get access item", http.StatusInternalServerError)
		slog.Error("get access item", "id", id, "err", err)
		return
	}
	writeJSON(w, item)
}

func (rt *routes) handleCreateAccessItem(w http.ResponseWriter, r *http.Request) {
	var req api.CreateAccessItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	item, err := rt.svc.CreateAccessItem(req)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
			return
		}
		writeError(w, ErrCodeInternal, "failed to create access item", http.StatusInternalServerError)
		slog.Error("create access item", "err", err)
		return
	}
	writeJSONCreated(w, item)
}

func (rt *routes) handleUpdateAccessItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	var req api.UpdateAccessItemRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	item, err := rt.svc.UpdateAccessItem(id, req)
	if err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, ErrCodeNotFound, "access item not found", http.StatusNotFound)
			return
		}
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
			return
		}
		writeError(w, ErrCodeInternal, "failed to update access item", http.StatusInternalServerError)
		slog.Error("update access item", "id", id, "err", err)
		return
	}
	writeJSON(w, item)
}

func (rt *routes) handleDeleteAccessItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := rt.svc.DeleteAccessItem(id); err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
			return
		}
		writeError(w, ErrCodeInternal, "failed to delete access item", http.StatusInternalServerError)
		slog.Error("delete access item", "id", id, "err", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (rt *routes) handleResetAccessItem(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	item, err := rt.svc.ResetAccessItem(id)
	if err != nil {
		if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
			return
		}
		writeError(w, ErrCodeInternal, "failed to reset access item", http.StatusInternalServerError)
		slog.Error("reset access item", "id", id, "err", err)
		return
	}
	writeJSON(w, item)
}

func (rt *routes) handleResolveAccessItems(w http.ResponseWriter, r *http.Request) {
	var req api.ResolveAccessItemsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	// No DB lookup — items are resolved directly from the request body.
	// Source commands in items execute on the host; access is limited by
	// the localhost-only CORS policy (same trust model as stored items).
	resp, err := rt.svc.ResolveAccessItems(req.Items)
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to resolve access items", http.StatusInternalServerError)
		slog.Error("resolve access items", "err", err)
		return
	}
	writeJSON(w, resp)
}

func (rt *routes) handleDefaults(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, rt.svc.GetDefaults())
}

// handleListRuntimes returns available container runtimes (Docker, Podman).
//
//	@Summary		List runtimes
//	@Description	Detects and returns available container runtimes (Docker, Podman) with
//	@Description	their socket paths and API versions.
//	@Tags			host
//	@Success		200	{array}	runtime.RuntimeInfo
//	@Router			/api/v1/runtimes [get]
func (rt *routes) handleListRuntimes(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, rt.svc.ListRuntimes(r.Context()))
}

// handleListDirectories returns filesystem entries at the given path for the
// browser in the Create Container dialog.
//
//	@Summary		List filesystem entries
//	@Description	Returns filesystem entries at the given path. By default only directories
//	@Description	are returned. Pass mode=file to include files alongside directories.
//	@Tags			host
//	@Param			path	query		string	true	"Absolute path to list entries in"
//	@Param			mode	query		string	false	"Browse mode: omit for directories only, 'file' for directories and files"
//	@Success		200		{array}		api.DirEntry
//	@Failure		400		{object}	apiError
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/filesystem/directories [get]
func (rt *routes) handleListDirectories(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		writeError(w, ErrCodeRequiredField, "path query parameter is required", http.StatusBadRequest)
		return
	}

	if !filepath.IsAbs(path) {
		writeError(w, ErrCodeInvalidPath, "path must be absolute", http.StatusBadRequest)
		return
	}

	includeFiles := r.URL.Query().Get("mode") == "file"

	dirs, err := rt.svc.ListDirectories(path, includeFiles)
	if err != nil {
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("list directories", "path", path, "err", err)
		return
	}

	writeJSON(w, dirs)
}

// handleRevealInFileManager opens the given directory in the host's file manager.
//
//	@Summary		Reveal in file manager
//	@Description	Opens the given host directory in the system file manager (Finder, Nautilus, Explorer).
//	@Tags			host
//	@Accept			json
//	@Param			body	body	revealRequest	true	"Directory path to reveal"
//	@Success		204		"Directory opened"
//	@Failure		400		{object}	apiError
//	@Failure		404		{object}	apiError	"Path not found"
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/filesystem/reveal [post]
func (rt *routes) handleRevealInFileManager(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if body.Path == "" {
		writeError(w, ErrCodeRequiredField, "path is required", http.StatusBadRequest)
		return
	}
	if !filepath.IsAbs(body.Path) {
		writeError(w, ErrCodeInvalidPath, "path must be absolute", http.StatusBadRequest)
		return
	}

	if err := rt.svc.RevealInFileManager(body.Path); err != nil {
		if errors.Is(err, service.ErrNotFound) {
			writeError(w, ErrCodeNotFound, "path not found", http.StatusNotFound)
		} else if errors.Is(err, service.ErrInvalidInput) {
			writeError(w, ErrCodeNotADirectory, "path is not a directory", http.StatusBadRequest)
		} else {
			slog.Warn("could not open file manager", "path", body.Path, "err", err)
			writeError(w, ErrCodeInternal, "failed to open file manager", http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

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

// handleTerminalWS upgrades to a WebSocket and bridges it to an abduco session
// inside the project's container via docker exec.
//
//	@Summary		Terminal WebSocket
//	@Description	Upgrades to a WebSocket connection and bridges it to an abduco terminal session
//	@Description	inside the container via docker exec. Binary frames carry raw PTY data;
//	@Description	text frames carry JSON control messages (e.g. {"type":"resize","cols":80,"rows":24}).
//	@Tags			streaming
//	@Param			projectId	path	string	true	"Project ID"
//	@Param			wid			path	string	true	"Worktree ID"
//	@Success		101			"WebSocket upgrade"
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		503			{object}	apiError	"Terminal proxy not configured"
//	@Router			/api/v1/projects/{projectId}/ws/{wid} [get]
func (rt *routes) handleTerminalWS(w http.ResponseWriter, r *http.Request) {
	row := rt.resolveProject(w, r)
	if row == nil {
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

// --- Input Validation ---

// isValidContainerID checks that an ID is a valid Docker container hex ID.
func isValidContainerID(id string) bool {
	return containerIDRegex.MatchString(id)
}

// isValidWorktreeID checks that a worktree ID matches the expected pattern.
func isValidWorktreeID(id string) bool {
	return engine.IsValidWorktreeID(id)
}

// isValidContainerName checks that a name matches Docker's container naming rules.
func isValidContainerName(name string) bool {
	return containerNameRegex.MatchString(name)
}

// isValidDomain checks that a domain string is a valid hostname.
func isValidDomain(domain string) bool {
	return len(domain) > 0 && len(domain) <= 253 && domainRegex.MatchString(domain)
}

// validateNetworkConfig checks network-related fields of a container
// request. Returns a non-empty error message on failure.
func validateNetworkConfig(req engine.CreateContainerRequest) string {
	if req.NetworkMode != "" && !engine.IsValidNetworkMode(string(req.NetworkMode)) {
		return "invalid network mode: must be full, restricted, or none"
	}
	if req.NetworkMode == engine.NetworkModeRestricted && len(req.AllowedDomains) == 0 {
		return "restricted network mode requires at least one allowed domain"
	}
	if len(req.AllowedDomains) > 0 && !validateAllowedDomains(req.AllowedDomains) {
		return "invalid domain in allowedDomains"
	}
	return ""
}

// validateAllowedDomains checks that all domain strings in the list are valid.
func validateAllowedDomains(domains []string) bool {
	for _, d := range domains {
		if !isValidDomain(d) {
			return false
		}
	}
	return true
}

// --- JSON Response Helpers ---

// writeJSONCreated marshals v to JSON and writes it with a 201 Created status.
func writeJSONCreated(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "err", err)
	}
}

// writeJSON marshals v to JSON and writes it to the response.
func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("json encode failed", "err", err)
	}
}

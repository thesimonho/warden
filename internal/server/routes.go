package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
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
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}", rt.handleRemoveProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/stop", rt.handleStopProject)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/restart", rt.handleRestartProject)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/worktrees", rt.handleListWorktrees)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees", rt.handleCreateWorktree)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect", rt.handleConnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect", rt.handleDisconnectTerminal)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill", rt.handleKillWorktreeProcess)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}", rt.handleRemoveWorktree)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff", rt.handleGetWorktreeDiff)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/worktrees/cleanup", rt.handleCleanupWorktrees)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/costs", rt.handleResetProjectCosts)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/audit", rt.handlePurgeProjectAudit)
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/container", rt.handleCreateContainer)
	mux.HandleFunc("DELETE /api/v1/projects/{projectId}/{agentType}/container", rt.handleDeleteContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/container/config", rt.handleInspectContainer)
	mux.HandleFunc("PUT /api/v1/projects/{projectId}/{agentType}/container", rt.handleUpdateContainer)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/container/validate", rt.handleValidateContainer)
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
	mux.HandleFunc("POST /api/v1/projects/{projectId}/{agentType}/clipboard", rt.handleUploadClipboard)
	mux.HandleFunc("GET /api/v1/projects/{projectId}/{agentType}/ws/{wid}", rt.handleTerminalWS)
}

// --- Helpers ---

// writeServiceError maps common service-layer errors to HTTP responses.
// Returns true if it wrote an error response, false if the error was not recognized.
func writeServiceError(w http.ResponseWriter, err error) bool {
	if errors.Is(err, service.ErrNotFound) {
		writeError(w, ErrCodeNotFound, "project not found", http.StatusNotFound)
		return true
	}
	if errors.Is(err, service.ErrInvalidInput) {
		writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
		return true
	}
	if errors.Is(err, service.ErrBudgetExceeded) {
		writeError(w, ErrCodeBudgetExceeded, err.Error(), http.StatusForbidden)
		return true
	}
	if engine.IsStaleMountsError(err) {
		writeError(w, ErrCodeStaleMounts, err.Error(), http.StatusConflict)
		return true
	}
	return false
}

// extractAgentType reads and validates the "agentType" path parameter.
// Returns the agent type and true if valid, or empty string and false
// if the value is not a recognized agent type.
func extractAgentType(r *http.Request) (string, bool) {
	at := constants.AgentType(r.PathValue("agentType"))
	if at.Valid() {
		return string(at), true
	}
	return "", false
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
		AgentType   string `json:"agentType"`
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

	if req.AgentType == "" {
		req.AgentType = string(constants.DefaultAgentType)
	}

	result, err := rt.svc.AddProject(req.Name, req.ProjectPath, req.AgentType)
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
//	@Router			/api/v1/projects/{projectId}/{agentType} [delete]
func (rt *routes) handleRemoveProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.RemoveProject(projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("remove project", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/costs [delete]
func (rt *routes) handleResetProjectCosts(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	if err := rt.svc.ResetProjectCosts(projectID, agentType); err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("reset project costs", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/audit [delete]
func (rt *routes) handlePurgeProjectAudit(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	deleted, err := rt.svc.PurgeProjectAudit(projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("purge project audit", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/stop [post]
func (rt *routes) handleStopProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.StopProject(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("stop project", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/restart [post]
func (rt *routes) handleRestartProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.RestartProject(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("restart project", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees [get]
func (rt *routes) handleListWorktrees(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	worktrees, err := rt.svc.ListWorktrees(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("list worktrees", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees [post]
func (rt *routes) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

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

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.CreateWorktree(r.Context(), projectID, agentType, req.Name)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("create worktree", "projectId", projectID, "name", req.Name, "err", err)
		return
	}

	writeJSONCreated(w, resp)
}

// handleConnectTerminal starts a terminal for a worktree in the given project.
//
//	@Summary		Connect terminal
//	@Description	Starts a tmux terminal session for the given worktree. If a background session
//	@Description	already exists, reconnects to it instead of creating a new one.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		201			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/connect [post]
func (rt *routes) handleConnectTerminal(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.ConnectTerminal(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("connect terminal", "projectId", projectID, "wid", wid, "err", err)
		return
	}

	writeJSONCreated(w, resp)
}

// handleDisconnectTerminal kills the terminal viewer for a worktree.
//
//	@Summary		Disconnect terminal
//	@Description	Closes the terminal viewer WebSocket. The tmux session (and Claude/bash)
//	@Description	continues running in the background.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/disconnect [post]
func (rt *routes) handleDisconnectTerminal(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.DisconnectTerminal(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("disconnect terminal", "projectId", projectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleKillWorktreeProcess kills the tmux session and all child processes for a worktree.
//
//	@Summary		Kill worktree process
//	@Description	Kills the tmux session and all child processes for the worktree.
//	@Description	The git worktree directory on disk is preserved. This is destructive —
//	@Description	any running Claude session is terminated immediately.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/kill [post]
func (rt *routes) handleKillWorktreeProcess(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.KillWorktreeProcess(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("kill worktree process", "projectId", projectID, "wid", wid, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid} [delete]
func (rt *routes) handleRemoveWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.RemoveWorktree(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("remove worktree", "projectId", projectID, "wid", wid, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleCleanupWorktrees removes orphaned worktree directories.
//
//	@Summary		Cleanup orphaned worktrees
//	@Description	Removes worktree directories that are not tracked by git, kills orphaned tmux
//	@Description	sessions, and prunes stale git worktree metadata.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	cleanupWorktreesResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/cleanup [post]
func (rt *routes) handleCleanupWorktrees(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	removed, err := rt.svc.CleanupWorktrees(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("cleanup worktrees", "projectId", projectID, "err", err)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/diff [get]
func (rt *routes) handleGetWorktreeDiff(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	wid := r.PathValue("wid")
	if !isValidWorktreeID(wid) {
		writeError(w, ErrCodeInvalidWorktreeID, "invalid worktree ID", http.StatusBadRequest)
		return
	}

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	resp, err := rt.svc.GetWorktreeDiff(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get worktree diff", "projectId", projectID, "wid", wid, "err", err)
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
//	@Param			body		body		api.CreateContainerRequest	true	"Container configuration"
//	@Success		201			{object}	service.ContainerResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		409			{object}	apiError	"Container name already in use"
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/container [post]
func (rt *routes) handleCreateContainer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req api.CreateContainerRequest
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
		slog.Error("create container", "projectId", projectID, "name", req.Name, "err", err)
		if writeServiceError(w, err) {
			return
		}
		if errors.Is(err, engine.ErrNameTaken) {
			writeError(w, ErrCodeNameTaken, err.Error(), http.StatusConflict)
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
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
//	@Router			/api/v1/projects/{projectId}/{agentType}/container [delete]
func (rt *routes) handleDeleteContainer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.DeleteContainer(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("delete container", "projectId", projectID, "err", err)
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
//	@Success		200			{object}	api.ContainerConfig
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/container/config [get]
func (rt *routes) handleInspectContainer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	cfg, err := rt.svc.InspectContainer(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("inspect container", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, cfg)
}

// handleValidateContainer checks whether the project's container has Warden terminal infrastructure.
//
//	@Summary		Validate container infrastructure
//	@Description	Checks whether the project's running container has the required Warden terminal
//	@Description	infrastructure installed (tmux, create-terminal.sh).
//	@Tags			containers
//	@Param			projectId	path		string	true	"Project ID"
//	@Success		200			{object}	validateContainerResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/container/validate [get]
func (rt *routes) handleValidateContainer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.ValidateContainer(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("validate infrastructure", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, validateContainerResponse{Valid: result.Valid, Missing: result.Missing})
}

// handleUpdateContainer updates the project's container configuration.
//
//	@Summary		Update container
//	@Description	Updates the project's container configuration. Lightweight changes (budget,
//	@Description	skip permissions, allowed domains) are applied in-place. Other changes (image, mounts,
//	@Description	env vars, network mode, agent type) trigger a full container recreation.
//	@Tags			containers
//	@Accept			json
//	@Param			projectId	path		string							true	"Project ID"
//	@Param			body		body		api.CreateContainerRequest	true	"Updated container configuration"
//	@Success		200			{object}	service.ContainerResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/container [put]
func (rt *routes) handleUpdateContainer(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req api.CreateContainerRequest
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

	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.UpdateContainer(r.Context(), projectID, agentType, req)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("update container", "projectId", projectID, "err", err)
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
//	@Success		200			{array}		api.AuditEntry
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
		entries = []api.AuditEntry{}
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

// handleListAccessItems returns all access items with detection status.
//
//	@Summary		List access items
//	@Description	Returns all access items (built-in + user-created) enriched with
//	@Description	per-credential host detection status. Built-in items that have been
//	@Description	customized via the DB are returned with the customized configuration.
//	@Tags			access
//	@Success		200	{object}	api.AccessItemListResponse
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/access [get]
func (rt *routes) handleListAccessItems(w http.ResponseWriter, _ *http.Request) {
	items, err := rt.svc.ListAccessItems()
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to list access items", http.StatusInternalServerError)
		slog.Error("list access items", "err", err)
		return
	}
	writeJSON(w, api.AccessItemListResponse{Items: items})
}

// handleGetAccessItem returns a single access item by ID.
//
//	@Summary		Get access item
//	@Description	Returns a single access item with detection status. For built-in items,
//	@Description	returns the DB override if one exists, otherwise the default configuration.
//	@Tags			access
//	@Param			id	path		string	true	"Access item ID"
//	@Success		200	{object}	api.AccessItemResponse
//	@Failure		404	{object}	apiError	"Item not found"
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/access/{id} [get]
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

// handleCreateAccessItem creates a user-defined access item.
//
//	@Summary		Create access item
//	@Description	Creates a new user-defined access item with the given label, description,
//	@Description	and credential configuration. Returns the created item with a generated ID.
//	@Tags			access
//	@Accept			json
//	@Param			body	body		api.CreateAccessItemRequest	true	"Access item configuration"
//	@Success		201		{object}	access.Item
//	@Failure		400		{object}	apiError	"Invalid input (missing label or credentials)"
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/access [post]
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

// handleUpdateAccessItem updates an access item's configuration.
//
//	@Summary		Update access item
//	@Description	Updates an access item. For built-in items, saves a customized copy to the DB
//	@Description	(overriding the default). For user items, updates the existing DB row.
//	@Description	Only provided fields are changed.
//	@Tags			access
//	@Accept			json
//	@Param			id		path		string						true	"Access item ID"
//	@Param			body	body		api.UpdateAccessItemRequest	true	"Fields to update (all optional)"
//	@Success		200		{object}	access.Item
//	@Failure		400		{object}	apiError	"Invalid input"
//	@Failure		404		{object}	apiError	"Item not found"
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/access/{id} [put]
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

// handleDeleteAccessItem removes a user-defined access item.
//
//	@Summary		Delete access item
//	@Description	Deletes a user-defined access item. Built-in items cannot be deleted —
//	@Description	use the reset endpoint instead.
//	@Tags			access
//	@Param			id	path	string	true	"Access item ID"
//	@Success		204	"No content"
//	@Failure		400	{object}	apiError	"Cannot delete built-in item"
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/access/{id} [delete]
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

// handleResetAccessItem restores a built-in item to its default configuration.
//
//	@Summary		Reset access item
//	@Description	Restores a built-in access item to its default configuration by removing
//	@Description	any DB override. Only works for built-in items (git, ssh).
//	@Tags			access
//	@Param			id	path		string	true	"Built-in access item ID"
//	@Success		200	{object}	access.Item
//	@Failure		400	{object}	apiError	"Not a built-in item"
//	@Failure		500	{object}	apiError
//	@Router			/api/v1/access/{id}/reset [post]
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

// handleResolveAccessItems resolves access items for test/preview.
//
//	@Summary		Resolve access items
//	@Description	Resolves the given access items by checking host sources and computing
//	@Description	the injections (env vars, mounts) that would be applied to containers.
//	@Description	Used by the UI "Test" button to preview resolution without creating a container.
//	@Tags			access
//	@Accept			json
//	@Param			body	body		api.ResolveAccessItemsRequest	true	"Item IDs to resolve"
//	@Success		200		{object}	api.ResolveAccessItemsResponse
//	@Failure		400		{object}	apiError	"Invalid request body"
//	@Failure		404		{object}	apiError	"Item not found"
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/access/resolve [post]
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

func (rt *routes) handleDefaults(w http.ResponseWriter, r *http.Request) {
	projectPath := r.URL.Query().Get("path")
	writeJSON(w, rt.svc.GetDefaults(projectPath))
}

// handleListRuntimes returns available container runtimes.
//
//	@Summary		List runtimes
//	@Description	Detects and returns available container runtimes with
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
func validateNetworkConfig(req api.CreateContainerRequest) string {
	if req.NetworkMode != "" && !api.IsValidNetworkMode(string(req.NetworkMode)) {
		return "invalid network mode: must be full, restricted, or none"
	}
	if req.NetworkMode == api.NetworkModeRestricted && len(req.AllowedDomains) == 0 {
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

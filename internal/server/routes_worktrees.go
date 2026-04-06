package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/thesimonho/warden/api"
)

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
//	@Param			projectId	path		string						true	"Project ID"
//	@Param			body		body		api.CreateWorktreeRequest	true	"Worktree name (must be a valid git branch name)"
//	@Success		201			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees [post]
func (rt *routes) handleCreateWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")

	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req api.CreateWorktreeRequest
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

// handleResetWorktree clears all history for a worktree without removing it.
//
//	@Summary		Reset worktree
//	@Description	Clears session state for a worktree: kills any running process, deletes agent
//	@Description	session files, and removes terminal tracking state. Audit events are preserved.
//	@Description	The worktree itself is preserved.
//	@Tags			worktrees
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			wid			path		string	true	"Worktree ID"
//	@Success		200			{object}	service.WorktreeResult
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/worktrees/{wid}/reset [post]
func (rt *routes) handleResetWorktree(w http.ResponseWriter, r *http.Request) {
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

	result, err := rt.svc.ResetWorktree(r.Context(), projectID, agentType, wid)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("reset worktree", "projectId", projectID, "wid", wid, "err", err)
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

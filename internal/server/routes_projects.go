package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/engine"
)

// handleListProjects returns all projects from the config, enriched with Docker state.
//
//	@Summary		List projects
//	@Description	Returns all configured projects enriched with live container state,
//	@Description	Claude status, worktree counts, and cost data.
//	@Tags			projects
//	@Success		200	{array}		api.ProjectResponse
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

// handleGetProject returns a single project by ID with full state.
//
//	@Summary		Get project
//	@Description	Returns a single project enriched with live container state,
//	@Description	Claude status, worktree counts, and cost data.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			agentType	path		string	true	"Agent type"
//	@Success		200			{object}	api.ProjectResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType} [get]
func (rt *routes) handleGetProject(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.GetProjectDetails(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get project", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleGetProjectCosts returns session-level cost data for a project.
//
//	@Summary		Get project costs
//	@Description	Returns session-level cost breakdown for the given project.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			agentType	path		string	true	"Agent type"
//	@Success		200			{object}	api.ProjectCostsResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/costs [get]
func (rt *routes) handleGetProjectCosts(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.GetProjectCosts(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get project costs", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleGetBudgetStatus returns the budget state for a project.
//
//	@Summary		Get budget status
//	@Description	Returns the effective budget, current cost, and over-budget state for a project.
//	@Tags			projects
//	@Param			projectId	path		string	true	"Project ID"
//	@Param			agentType	path		string	true	"Agent type"
//	@Success		200			{object}	api.BudgetStatusResponse
//	@Failure		400			{object}	apiError
//	@Failure		404			{object}	apiError
//	@Failure		500			{object}	apiError
//	@Router			/api/v1/projects/{projectId}/{agentType}/budget [get]
func (rt *routes) handleGetBudgetStatus(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	agentType, ok := extractAgentType(r)
	if !ok {
		writeError(w, ErrCodeInvalidBody, "invalid agent type", http.StatusBadRequest)
		return
	}

	result, err := rt.svc.GetBudgetStatus(r.Context(), projectID, agentType)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		writeError(w, ErrCodeInternal, err.Error(), http.StatusInternalServerError)
		slog.Error("get budget status", "projectId", projectID, "err", err)
		return
	}

	writeJSON(w, result)
}

// handleBatchProjects performs an action on multiple projects.
//
//	@Summary		Batch project operation
//	@Description	Performs an action (stop, restart, delete) on multiple projects.
//	@Description	Each project is processed independently — failures are per-item.
//	@Tags			projects
//	@Accept			json
//	@Param			body	body		api.BatchProjectRequest	true	"Batch operation"
//	@Success		200		{object}	api.BatchProjectResponse
//	@Failure		400		{object}	apiError
//	@Router			/api/v1/projects/batch [post]
func (rt *routes) handleBatchProjects(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req api.BatchProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	switch req.Action {
	case api.BatchActionStop, api.BatchActionRestart, api.BatchActionDelete:
		// valid
	default:
		writeError(w, ErrCodeInvalidBody, "action must be stop, restart, or delete", http.StatusBadRequest)
		return
	}

	if len(req.Projects) == 0 {
		writeError(w, ErrCodeRequiredField, "projects list is required", http.StatusBadRequest)
		return
	}

	if len(req.Projects) > 50 {
		writeError(w, ErrCodeInvalidBody, "batch size limit is 50 projects", http.StatusBadRequest)
		return
	}

	result := rt.svc.BatchProjectOperation(r.Context(), req)
	writeJSON(w, result)
}

// handleAddProject registers a project directory in Warden. When the request
// includes a "container" field, the container is created atomically.
//
//	@Summary		Add project
//	@Description	Registers a host directory as a Warden project. Optionally creates a
//	@Description	container in the same request by including a "container" field. If
//	@Description	container creation fails, the project is cleaned up automatically.
//	@Tags			projects
//	@Accept			json
//	@Param			body	body		api.AddProjectRequest	true	"Project details (with optional container config)"
//	@Success		201		{object}	api.AddProjectResponse
//	@Failure		400		{object}	apiError
//	@Failure		409		{object}	apiError	"Container name already in use"
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/projects [post]
func (rt *routes) handleAddProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<16)

	var req api.AddProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" && req.CloneURL == "" {
		writeError(w, ErrCodeRequiredField, "projectPath or cloneURL is required", http.StatusBadRequest)
		return
	}
	if req.ProjectPath != "" && req.CloneURL != "" {
		writeError(w, ErrCodeInvalidBody, "provide only one of projectPath or cloneURL", http.StatusBadRequest)
		return
	}

	if req.Name != "" && !isValidContainerName(req.Name) {
		writeError(w, ErrCodeInvalidContainerName, "invalid container name", http.StatusBadRequest)
		return
	}

	if req.AgentType == "" {
		req.AgentType = string(constants.DefaultAgentType)
	}

	if req.Container != nil {
		if msg := validateNetworkConfig(*req.Container); msg != "" {
			writeError(w, ErrCodeInvalidNetworkConfig, msg, http.StatusBadRequest)
			return
		}
	}

	result, err := rt.svc.AddProjectWithContainer(r.Context(), req)
	if err != nil {
		if writeServiceError(w, err) {
			return
		}
		if errors.Is(err, engine.ErrNameTaken) {
			writeError(w, ErrCodeNameTaken, err.Error(), http.StatusConflict)
			return
		}
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

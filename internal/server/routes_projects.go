package server

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
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

// handleAddProject registers a project directory in Warden.
//
//	@Summary		Add project
//	@Description	Registers a host directory as a Warden project.
//	@Tags			projects
//	@Accept			json
//	@Param			body	body		api.AddProjectRequest	true	"Project details"
//	@Success		201		{object}	service.ProjectResult
//	@Failure		400		{object}	apiError
//	@Failure		500		{object}	apiError
//	@Router			/api/v1/projects [post]
func (rt *routes) handleAddProject(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req api.AddProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProjectPath == "" {
		writeError(w, ErrCodeRequiredField, "projectPath is required", http.StatusBadRequest)
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

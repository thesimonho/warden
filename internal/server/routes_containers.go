package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
)

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

	if msg := validateProjectSource(req.ProjectPath, req.CloneURL); msg != "" {
		writeError(w, ErrCodeInvalidBody, msg, http.StatusBadRequest)
		return
	}

	if msg := validateNetworkConfig(req); msg != "" {
		writeError(w, ErrCodeInvalidNetworkConfig, msg, http.StatusBadRequest)
		return
	}
	if msg := validateForwardedPorts(req.ForwardedPorts); msg != "" {
		writeError(w, ErrCodeInvalidBody, msg, http.StatusBadRequest)
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

	if msg := validateProjectSource(req.ProjectPath, req.CloneURL); msg != "" {
		writeError(w, ErrCodeInvalidBody, msg, http.StatusBadRequest)
		return
	}

	if msg := validateNetworkConfig(req); msg != "" {
		writeError(w, ErrCodeInvalidNetworkConfig, msg, http.StatusBadRequest)
		return
	}
	if msg := validateForwardedPorts(req.ForwardedPorts); msg != "" {
		writeError(w, ErrCodeInvalidBody, msg, http.StatusBadRequest)
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

package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"github.com/thesimonho/warden/service"
)

// --- Settings ---

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

// --- Host Utilities ---

// handleDefaults returns server-resolved default values for the create container form.
//
//	@Summary		Get defaults
//	@Description	Returns server-resolved default values for the create container form,
//	@Description	including the host home directory and auto-detected bind mounts.
//	@Tags			host
//	@Success		200	{object}	service.DefaultsResponse
//	@Router			/api/v1/defaults [get]
func (rt *routes) handleDefaults(w http.ResponseWriter, r *http.Request) {
	projectPath := r.URL.Query().Get("path")
	writeJSON(w, rt.svc.GetDefaults(projectPath))
}

// handleReadTemplate reads a .warden.json project template from an arbitrary path.
//
//	@Summary		Read project template
//	@Description	Reads and parses a .warden.json file from the given path.
//	@Description	Used to import templates from outside the project directory.
//	@Tags			host
//	@Param			path	query		string	true	"Absolute path to .warden.json"
//	@Success		200		{object}	api.ProjectTemplate
//	@Failure		400		{object}	apiError
//	@Failure		404		{object}	apiError
//	@Router			/api/v1/template [get]
func (rt *routes) handleReadTemplate(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeError(w, ErrCodeRequiredField, "path query parameter is required", http.StatusBadRequest)
		return
	}
	tmpl, err := rt.svc.ReadProjectTemplate(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			writeError(w, ErrCodeNotFound, "template not found", http.StatusNotFound)
			return
		}
		writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, tmpl)
}

// handleValidateTemplate validates and sanitizes a .warden.json template body.
//
//	@Summary		Validate project template
//	@Description	Accepts a raw .warden.json body, validates it against the ProjectTemplate
//	@Description	schema, applies security sanitization, and returns the cleaned template.
//	@Description	Used by the frontend import-from-file flow.
//	@Tags			host
//	@Accept			json
//	@Param			body	body		api.ProjectTemplate	true	"Raw template JSON"
//	@Success		200		{object}	api.ProjectTemplate
//	@Failure		400		{object}	apiError
//	@Router			/api/v1/template [post]
func (rt *routes) handleValidateTemplate(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10) // 64 KiB limit

	data, err := io.ReadAll(r.Body)
	if err != nil {
		writeError(w, ErrCodeInvalidBody, "failed to read request body", http.StatusBadRequest)
		return
	}

	tmpl, err := rt.svc.ValidateProjectTemplate(data)
	if err != nil {
		writeError(w, ErrCodeInvalidBody, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, tmpl)
}

// handleListRuntimes returns available container runtimes.
//
//	@Summary		List runtimes
//	@Description	Detects and returns available container runtimes with
//	@Description	their socket paths and API versions.
//	@Tags			host
//	@Success		200	{object}	docker.Info
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

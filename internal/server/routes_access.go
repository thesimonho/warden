package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/service"
)

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
//	@Description	any DB override. Only works for built-in items (git, ssh, gpg).
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

	resp, err := rt.svc.ResolveAccessItems(req.Items)
	if err != nil {
		writeError(w, ErrCodeInternal, "failed to resolve access items", http.StatusInternalServerError)
		slog.Error("resolve access items", "err", err)
		return
	}
	writeJSON(w, resp)
}

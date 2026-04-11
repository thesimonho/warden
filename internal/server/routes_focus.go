package server

import (
	"encoding/json"
	"net/http"

	"github.com/thesimonho/warden/api"
)

// --- Focus ---

// handleReportFocus accepts a client's focus state report.
//
//	@Summary		Report focus state
//	@Description	Reports which project and worktrees a client is actively viewing.
//	@Description	Used by the system tray to suppress desktop notifications for focused projects.
//	@Tags			focus
//	@Accept			json
//	@Param			body	body	api.FocusRequest	true	"Focus state report"
//	@Success		204
//	@Failure		400	{object}	apiError
//	@Router			/api/v1/focus [post]
func (rt *routes) handleReportFocus(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 1<<10)

	var req api.FocusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, ErrCodeInvalidBody, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ClientID == "" {
		writeError(w, ErrCodeInvalidBody, "clientId is required", http.StatusBadRequest)
		return
	}

	rt.svc.ReportFocus(req)
	w.WriteHeader(http.StatusNoContent)
}

// handleGetFocusState returns the aggregated viewer focus state.
//
//	@Summary		Get focus state
//	@Description	Returns the aggregated viewer focus state across all connected clients.
//	@Tags			focus
//	@Success		200	{object}	api.FocusState
//	@Router			/api/v1/focus [get]
func (rt *routes) handleGetFocusState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, rt.svc.GetFocusState())
}

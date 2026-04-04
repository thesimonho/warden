package server

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/service"
)

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

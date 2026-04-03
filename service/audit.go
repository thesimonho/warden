package service

import (
	"context"
	"encoding/csv"
	"fmt"
	"io"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/eventbus"
)

// auditEventsByCategory maps audit categories to their corresponding event types.
// Uses eventbus constants to avoid string literal drift.
var auditEventsByCategory = map[api.AuditCategory][]string{
	api.AuditCategorySession: {
		string(eventbus.EventSessionStart), string(eventbus.EventSessionEnd),
		string(eventbus.EventSessionExit),
		string(eventbus.EventTurnComplete), string(eventbus.EventTurnDuration),
		string(eventbus.EventContextCompact),
		string(eventbus.EventSystemInfo),
		// Terminal lifecycle.
		string(eventbus.EventTerminalConnected), string(eventbus.EventTerminalDisconnected),
		"container_heartbeat_stale", "container_startup_failed",
		// Worktree lifecycle.
		"worktree_created", "worktree_removed", "worktree_cleaned_up",
		"worktree_create_failed", "terminal_connect_failed", "terminal_disconnect_failed",
		"worktree_kill_failed", "worktree_remove_failed", "worktree_cleanup_failed",
		string(eventbus.EventStopFailure),
	},
	api.AuditCategoryAgent: {
		string(eventbus.EventToolUse),
		string(eventbus.EventToolUseFailure), string(eventbus.EventPermissionRequest),
		string(eventbus.EventSubagentStart), string(eventbus.EventSubagentStop),
		string(eventbus.EventTaskCompleted),
		string(eventbus.EventElicitation), string(eventbus.EventElicitationResult),
		string(eventbus.EventPermissionGrant),
	},
	api.AuditCategoryPrompt: {string(eventbus.EventUserPrompt)},
	api.AuditCategoryConfig: {
		string(eventbus.EventConfigChange), string(eventbus.EventInstructionsLoaded),
	},
	api.AuditCategoryBudget: {
		"budget_exceeded", "budget_worktrees_stopped",
		"budget_container_stopped", "budget_enforcement_failed",
		"cost_reset",
	},
	api.AuditCategorySystem: {
		string(eventbus.EventProcessKilled), "restart_blocked_stale_mounts",
		"project_removed", "container_deleted", "audit_purged",
		"access_item_created", "access_item_updated", "access_item_deleted", "access_item_reset",
		string(eventbus.EventApiMetrics),
	},
}

// eventCategoryLookup maps event names to their audit category.
// Derived from auditEventsByCategory at init. Events not in the map
// fall into the "debug" category (auto-captured slog events).
var eventCategoryLookup = func() map[string]api.AuditCategory {
	m := make(map[string]api.AuditCategory)
	for cat, events := range auditEventsByCategory {
		for _, event := range events {
			m[event] = cat
		}
	}
	return m
}()

// allMappedEventNames contains every event name in any category map.
// Computed once at init for use as the "debug" category exclusion list.
var allMappedEventNames = func() []string {
	names := make([]string, 0, len(eventCategoryLookup))
	for event := range eventCategoryLookup {
		names = append(names, event)
	}
	return names
}()

// StandardAuditEvents returns the set of event names logged in standard
// audit mode. Derived from auditEventsByCategory using the standard
// categories defined in api.StandardAuditCategories. This is the single
// source of truth — pass the result to db.NewAuditWriter.
func StandardAuditEvents() map[string]bool {
	events := make(map[string]bool)
	for _, cat := range api.StandardAuditCategories {
		for _, event := range auditEventsByCategory[cat] {
			events[event] = true
		}
	}
	return events
}

// categoryForEvent returns the audit category for an event name.
// Returns "debug" for any event not explicitly mapped (e.g. slog-generated events).
func categoryForEvent(event string) api.AuditCategory {
	if cat, ok := eventCategoryLookup[event]; ok {
		return cat
	}
	return api.AuditCategoryDebug
}

// populateCategories sets the Category field on each entry.
func populateCategories(entries []db.Entry) {
	for i := range entries {
		entries[i].Category = string(categoryForEvent(entries[i].Event))
	}
}

// GetAuditLog returns filtered events from the audit log.
// When a category is specified, only matching event types are returned.
// When no category is specified, all events are returned.
// Each entry includes its computed audit category.
func (s *Service) GetAuditLog(filters api.AuditFilters) ([]db.Entry, error) {
	if s.db == nil {
		return []db.Entry{}, nil
	}

	qf := buildAuditQueryFilters(filters)
	entries, err := s.db.Query(qf)
	if err != nil {
		return nil, err
	}
	populateCategories(entries)
	return entries, nil
}

// GetAuditSummary returns aggregate statistics for audit events.
func (s *Service) GetAuditSummary(_ context.Context, filters api.AuditFilters) (*api.AuditSummary, error) {
	if s.db == nil {
		return &api.AuditSummary{TopTools: []api.ToolCount{}}, nil
	}

	qf := buildAuditQueryFilters(filters)

	row, err := s.db.QueryAuditSummary(qf)
	if err != nil {
		return nil, err
	}

	topToolRows, err := s.db.QueryTopTools(qf, 10)
	if err != nil {
		return nil, err
	}

	topTools := make([]api.ToolCount, len(topToolRows))
	for i, t := range topToolRows {
		topTools[i] = api.ToolCount{Name: t.Name, Count: t.Count}
	}

	// Cost aggregation: always query session_costs directly so the total
	// includes costs from deleted projects (preserved when audit logging is on).
	// GetCostInTimeRange handles zero times as open bounds.
	var totalCost float64
	costRow, costErr := s.db.GetCostInTimeRange(filters.ProjectID, qf.Since, qf.Until)
	if costErr == nil {
		totalCost = costRow.TotalCost
	}

	return &api.AuditSummary{
		TotalSessions:   row.TotalSessions,
		TotalToolUses:   row.TotalToolUses,
		TotalPrompts:    row.TotalPrompts,
		TotalCostUSD:    totalCost,
		UniqueProjects:  row.UniqueProjects,
		UniqueWorktrees: row.UniqueWorktrees,
		TopTools:        topTools,
		TimeRange: api.TimeRange{
			Earliest: row.Earliest,
			Latest:   row.Latest,
		},
	}, nil
}

// WriteAuditCSV writes audit entries as CSV to the given writer.
func (s *Service) WriteAuditCSV(w io.Writer, filters api.AuditFilters) error {
	entries, err := s.GetAuditLog(filters)
	if err != nil {
		return err
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	header := []string{"timestamp", "source", "level", "event", "projectId", "containerName", "worktree", "message", "data"}
	if err := cw.Write(header); err != nil {
		return fmt.Errorf("writing CSV header: %w", err)
	}

	for _, e := range entries {
		dataStr := ""
		if len(e.Data) > 0 {
			dataStr = string(e.Data)
		}

		record := []string{
			e.Timestamp.Format(time.RFC3339),
			string(e.Source),
			string(e.Level),
			e.Event,
			e.ProjectID,
			e.ContainerName,
			e.Worktree,
			e.Message,
			dataStr,
		}
		if err := cw.Write(record); err != nil {
			return fmt.Errorf("writing CSV record: %w", err)
		}
	}

	return nil
}

// GetAuditProjects returns distinct project (container) names from the audit log.
func (s *Service) GetAuditProjects() ([]string, error) {
	if s.db == nil {
		return []string{}, nil
	}
	return s.db.DistinctProjectIDs()
}

// PostAuditEvent writes a frontend-posted event to the audit log.
func (s *Service) PostAuditEvent(req api.PostAuditEventRequest) error {
	if s.db == nil {
		return nil
	}

	level := db.Level(req.Level)
	if level == "" {
		level = db.LevelInfo
	}

	entry := db.Entry{
		Source:  db.SourceFrontend,
		Level:   level,
		Event:   req.Event,
		Message: req.Message,
		Attrs:   req.Attrs,
	}
	s.audit.Write(entry)
	return nil
}

// DeleteAuditEvents removes events matching the given filters.
// With no filters, clears all events.
func (s *Service) DeleteAuditEvents(filters api.AuditFilters) (int64, error) {
	if s.db == nil {
		return 0, nil
	}

	qf := buildAuditQueryFilters(filters)
	return s.db.Delete(qf)
}

// buildAuditQueryFilters converts audit-specific filters to db.QueryFilters.
func buildAuditQueryFilters(filters api.AuditFilters) db.QueryFilters {
	var sinceTime, untilTime time.Time
	if filters.Since != "" {
		if parsed, err := time.Parse(time.RFC3339, filters.Since); err == nil {
			sinceTime = parsed
		}
	}
	if filters.Until != "" {
		if parsed, err := time.Parse(time.RFC3339, filters.Until); err == nil {
			untilTime = parsed
		}
	}

	// Determine which events to include based on category.
	// For "debug", exclude all explicitly mapped events so only unmapped
	// (slog-generated) events remain.
	var events []string
	var excludeEvents []string
	if filters.Category != "" {
		if catEvents, ok := auditEventsByCategory[filters.Category]; ok {
			events = catEvents
		} else if filters.Category == api.AuditCategoryDebug {
			excludeEvents = allMappedEventNames
		}
	}
	// When no category is specified, return all events (no event filter).

	return db.QueryFilters{
		Source:        db.Source(filters.Source),
		Level:         db.Level(filters.Level),
		ProjectID:     filters.ProjectID,
		Worktree:      filters.Worktree,
		Events:        events,
		ExcludeEvents: excludeEvents,
		Since:         sinceTime,
		Until:         untilTime,
		Limit:         filters.Limit,
		Offset:        filters.Offset,
	}
}

package service

import (
	"context"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
)

func newTestService(t *testing.T) (*Service, *db.Store) {
	t.Helper()
	store := newTestStore(t)
	return New(ServiceDeps{DockerAvailable: true, DB: store}), store
}

func newTestStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New() error: %v", err)
	}
	t.Cleanup(func() { store.Close() }) //nolint:errcheck
	return store
}

func writeTestEvents(t *testing.T, store *db.Store) {
	t.Helper()
	events := []db.Entry{
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "session_start", ProjectID: "proj1", Worktree: "main"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "tool_use", ProjectID: "proj1", Worktree: "main", Message: "Read"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "tool_use", ProjectID: "proj1", Worktree: "main", Message: "Edit"},
		{Source: db.SourceAgent, Level: db.LevelError, Event: "tool_use_failure", ProjectID: "proj1", Worktree: "main", Message: "Bash"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "permission_request", ProjectID: "proj1", Worktree: "main", Message: "Write"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "user_prompt", ProjectID: "proj1", Worktree: "main", Message: "fix the bug"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "tool_use", ProjectID: "proj1", Worktree: "feat-1", Message: "Read"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "session_end", ProjectID: "proj1", Worktree: "main"},
		{Source: db.SourceAgent, Level: db.LevelWarn, Event: "stop_failure", ProjectID: "proj1", Worktree: "main", Message: "rate_limit"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "subagent_start", ProjectID: "proj1", Worktree: "main", Message: "Explore"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "subagent_stop", ProjectID: "proj1", Worktree: "main", Message: "Explore"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "task_completed", ProjectID: "proj1", Worktree: "main", Message: "Fix bug"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "config_change", ProjectID: "proj1", Worktree: "main", Message: "user_settings"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "instructions_loaded", ProjectID: "proj1", Worktree: "main", Message: "CLAUDE.md"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "elicitation", ProjectID: "proj1", Worktree: "main", Message: "github"},
		{Source: db.SourceAgent, Level: db.LevelInfo, Event: "elicitation_result", ProjectID: "proj1", Worktree: "main", Message: "github"},
		{Source: db.SourceContainer, Level: db.LevelInfo, Event: "terminal_connected", ProjectID: "proj2", Worktree: "main"},
		{Source: db.SourceBackend, Level: db.LevelInfo, Event: "container_create", ProjectID: "proj2", Worktree: "", Message: "created container"},
	}
	for _, e := range events {
		if err := store.Write(e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}
}

func TestGetAuditLog_AllEvents(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	entries, err := svc.GetAuditLog(api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}

	// No category filter = all 18 events returned.
	if len(entries) != 18 {
		t.Errorf("expected 18 entries, got %d", len(entries))
	}
}

func TestGetAuditLog_AgentCategoryFilter(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategoryAgent})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}

	agentEvents := map[string]bool{
		"tool_use": true, "tool_use_failure": true,
		"permission_request": true, "subagent_start": true, "subagent_stop": true,
		"task_completed": true, "elicitation": true, "elicitation_result": true,
	}
	for _, e := range entries {
		if !agentEvents[e.Event] {
			t.Errorf("unexpected event type for agent category: %s", e.Event)
		}
	}

	// tool_use(3) + tool_use_failure(1) + permission_request(1)
	// + subagent_start(1) + subagent_stop(1) + task_completed(1)
	// + elicitation(1) + elicitation_result(1) = 10
	if len(entries) != 10 {
		t.Errorf("expected 10 agent entries, got %d", len(entries))
	}
}

func TestGetAuditLog_ContainerFilter(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	entries, err := svc.GetAuditLog(api.AuditFilters{ProjectID: "proj2"})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}

	for _, e := range entries {
		if e.ProjectID != "proj2" {
			t.Errorf("expected container proj2, got %s", e.ProjectID)
		}
	}

	// proj2 has terminal_connected + container_create = 2 events
	if len(entries) != 2 {
		t.Errorf("expected 2 entries for proj2, got %d", len(entries))
	}
}

func TestGetAuditLog_WorktreeFilter(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	entries, err := svc.GetAuditLog(api.AuditFilters{
		Category: api.AuditCategoryAgent,
		Worktree: "feat-1",
	})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}

	if len(entries) != 1 {
		t.Errorf("expected 1 tool entry for feat-1, got %d", len(entries))
	}
}

func TestGetAuditLog_ConfigCategory(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategoryConfig})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}

	configEvents := map[string]bool{"config_change": true, "instructions_loaded": true}
	for _, e := range entries {
		if !configEvents[e.Event] {
			t.Errorf("unexpected event type for config category: %s", e.Event)
		}
	}
	// config_change(1) + instructions_loaded(1) = 2
	if len(entries) != 2 {
		t.Errorf("expected 2 config entries, got %d", len(entries))
	}
}

func TestGetAuditSummary(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	summary, err := svc.GetAuditSummary(context.Background(), api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditSummary() error: %v", err)
	}

	if summary.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", summary.TotalSessions)
	}
	if summary.TotalToolUses != 3 {
		t.Errorf("expected 3 tool uses, got %d", summary.TotalToolUses)
	}
	if summary.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt, got %d", summary.TotalPrompts)
	}
	if summary.UniqueProjects < 2 {
		t.Errorf("expected at least 2 unique projects, got %d", summary.UniqueProjects)
	}
	if summary.UniqueWorktrees < 2 {
		t.Errorf("expected at least 2 unique worktrees, got %d", summary.UniqueWorktrees)
	}

	// Top tools should include Read and Edit.
	if len(summary.TopTools) == 0 {
		t.Error("expected at least one top tool")
	}
	foundRead := false
	for _, tc := range summary.TopTools {
		if tc.Name == "Read" {
			foundRead = true
			if tc.Count != 2 {
				t.Errorf("expected Read count 2, got %d", tc.Count)
			}
		}
	}
	if !foundRead {
		t.Error("expected Read in top tools")
	}
}

func TestGetAuditSummary_TimeFilteredCost(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	_ = store.UpsertSessionCost("proj1", "claude-code", "s1", 1.50, false)
	_ = store.UpsertSessionCost("proj1", "claude-code", "s2", 3.00, false)

	t.Run("range encompassing now includes all costs", func(t *testing.T) {
		summary, err := svc.GetAuditSummary(context.Background(), api.AuditFilters{
			Since: "2020-01-01T00:00:00Z",
			Until: "2030-01-01T00:00:00Z",
		})
		if err != nil {
			t.Fatalf("GetAuditSummary() error: %v", err)
		}
		if summary.TotalCostUSD != 4.50 {
			t.Errorf("expected time-filtered cost 4.50, got %.2f", summary.TotalCostUSD)
		}
	})

	t.Run("range entirely in past returns zero cost", func(t *testing.T) {
		summary, err := svc.GetAuditSummary(context.Background(), api.AuditFilters{
			Since: "2020-01-01T00:00:00Z",
			Until: "2020-01-02T00:00:00Z",
		})
		if err != nil {
			t.Fatalf("GetAuditSummary() error: %v", err)
		}
		if summary.TotalCostUSD != 0 {
			t.Errorf("expected 0 cost for past range, got %.2f", summary.TotalCostUSD)
		}
	})
}

func TestGetAuditSummary_DeletedProjectCost(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)

	// Simulate cost data from a project that was later deleted.
	// The session_costs rows are preserved (audit logging on), but the
	// project no longer exists in ListProjects.
	_ = store.UpsertSessionCost("deleted-proj", "claude-code", "s1", 2.50, false)
	_ = store.UpsertSessionCost("deleted-proj", "claude-code", "s2", 1.00, false)

	// Without time filter — this was the buggy path.
	summary, err := svc.GetAuditSummary(context.Background(), api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditSummary() error: %v", err)
	}
	if summary.TotalCostUSD != 3.50 {
		t.Errorf("expected cost 3.50 from deleted project, got %.2f", summary.TotalCostUSD)
	}
}

func TestGetAuditSummary_EmptyDB(t *testing.T) {
	t.Parallel()
	svc, _ := newTestService(t)

	summary, err := svc.GetAuditSummary(context.Background(), api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditSummary() error: %v", err)
	}

	if summary.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", summary.TotalSessions)
	}
	if summary.TopTools == nil {
		t.Error("TopTools should be non-nil empty slice")
	}
}

func TestWriteAuditCSV(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)
	writeTestEvents(t, store)

	var buf []byte
	w := &bytesWriter{buf: &buf}
	err := svc.WriteAuditCSV(w, api.AuditFilters{})
	if err != nil {
		t.Fatalf("WriteAuditCSV() error: %v", err)
	}

	output := string(*w.buf)
	if len(output) == 0 {
		t.Error("expected non-empty CSV output")
	}

	// Should start with a CSV header.
	if output[:9] != "timestamp" {
		t.Errorf("expected CSV to start with 'timestamp', got %q", output[:9])
	}
}

func TestGetAuditLog_NilDB(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{DockerAvailable: true})

	entries, err := svc.GetAuditLog(api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries with nil db, got %d", len(entries))
	}
}

func TestStandardAuditEvents(t *testing.T) {
	t.Parallel()
	events := StandardAuditEvents()

	// Must contain all session events.
	sessionEvents := []string{
		"session_start", "session_end", "session_exit",
		"terminal_connected", "terminal_disconnected",
		"stop_failure", "worktree_created", "worktree_removed",
	}
	for _, name := range sessionEvents {
		if !events[name] {
			t.Errorf("StandardAuditEvents() missing session event %q", name)
		}
	}

	// Must contain all budget events.
	budgetEvents := []string{
		"budget_exceeded", "budget_worktrees_stopped",
		"budget_container_stopped", "budget_enforcement_failed",
	}
	for _, name := range budgetEvents {
		if !events[name] {
			t.Errorf("StandardAuditEvents() missing budget event %q", name)
		}
	}

	// Must contain system events.
	systemEvents := []string{"process_killed", "restart_blocked_stale_mounts"}
	for _, name := range systemEvents {
		if !events[name] {
			t.Errorf("StandardAuditEvents() missing system event %q", name)
		}
	}

	// Must NOT contain detailed-only events.
	detailedOnly := []string{"tool_use", "tool_result", "user_prompt", "config_change"}
	for _, name := range detailedOnly {
		if events[name] {
			t.Errorf("StandardAuditEvents() should not contain detailed-only event %q", name)
		}
	}
}

func TestGetAuditLog_BudgetCategoryFilter(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)

	// Write a budget event.
	if err := store.Write(db.Entry{
		Source:    db.SourceBackend,
		Level:     db.LevelError,
		ProjectID: "proj1",
		Event:     "budget_exceeded",
		Message:   "cost $15.00 exceeds budget $10.00",
	}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategoryBudget})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 budget entry, got %d", len(entries))
	}
	if entries[0].Event != "budget_exceeded" {
		t.Errorf("expected event budget_exceeded, got %q", entries[0].Event)
	}
	if entries[0].Category != "budget" {
		t.Errorf("expected category budget, got %q", entries[0].Category)
	}
}

func TestPostAuditEvent_DefaultsToExternal(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	audit := db.NewAuditWriter(store, db.AuditDetailed, StandardAuditEvents())
	svc := New(ServiceDeps{DockerAvailable: true, DB: store, Audit: audit})

	err := svc.PostAuditEvent(api.PostAuditEventRequest{
		Event:   "deployment_started",
		Message: "deploying v1.2.3",
	})
	if err != nil {
		t.Fatalf("PostAuditEvent() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != db.SourceExternal {
		t.Errorf("expected source external, got %q", entries[0].Source)
	}
	if entries[0].Event != "deployment_started" {
		t.Errorf("expected event deployment_started, got %q", entries[0].Event)
	}
}

func TestPostAuditEvent_CustomSourceAndProject(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	audit := db.NewAuditWriter(store, db.AuditDetailed, StandardAuditEvents())
	svc := New(ServiceDeps{DockerAvailable: true, DB: store, Audit: audit})

	err := svc.PostAuditEvent(api.PostAuditEventRequest{
		Event:     "custom_event",
		Source:    "frontend",
		Level:     "warn",
		ProjectID: "aabbccddee01",
		AgentType: "claude-code",
		Worktree:  "main",
		Data:      []byte(`{"key":"value"}`),
		Attrs:     map[string]any{"version": "1.0"},
	})
	if err != nil {
		t.Fatalf("PostAuditEvent() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	e := entries[0]
	if e.Source != db.SourceFrontend {
		t.Errorf("source = %q, want frontend", e.Source)
	}
	if e.Level != db.LevelWarn {
		t.Errorf("level = %q, want warn", e.Level)
	}
	if e.ProjectID != "aabbccddee01" {
		t.Errorf("projectID = %q, want aabbccddee01", e.ProjectID)
	}
	if e.AgentType != "claude-code" {
		t.Errorf("agentType = %q, want claude-code", e.AgentType)
	}
	if e.Worktree != "main" {
		t.Errorf("worktree = %q, want main", e.Worktree)
	}
	if string(e.Data) != `{"key":"value"}` {
		t.Errorf("data = %q, want {\"key\":\"value\"}", string(e.Data))
	}
}

func TestPostAuditEvent_InvalidSourceFallsBackToExternal(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	audit := db.NewAuditWriter(store, db.AuditDetailed, StandardAuditEvents())
	svc := New(ServiceDeps{DockerAvailable: true, DB: store, Audit: audit})

	err := svc.PostAuditEvent(api.PostAuditEventRequest{
		Event:  "test_event",
		Source: "bogus",
	})
	if err != nil {
		t.Fatalf("PostAuditEvent() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Source != db.SourceExternal {
		t.Errorf("expected fallback to external, got %q", entries[0].Source)
	}
}

func TestPostAuditEvent_StandardModePassesExternalEvents(t *testing.T) {
	t.Parallel()
	store := newTestStore(t)
	audit := db.NewAuditWriter(store, db.AuditStandard, StandardAuditEvents())
	svc := New(ServiceDeps{DockerAvailable: true, DB: store, Audit: audit})

	// Custom event from external source — should pass even in standard mode.
	err := svc.PostAuditEvent(api.PostAuditEventRequest{
		Event:   "custom_integrator_event",
		Message: "should not be filtered",
	})
	if err != nil {
		t.Fatalf("PostAuditEvent() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry in standard mode, got %d", len(entries))
	}
}

func TestGetAuditLog_DebugCategoryFilter(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)

	// Write an unmapped event (will fall into debug category).
	if err := store.Write(db.Entry{
		Source:  db.SourceBackend,
		Level:   db.LevelWarn,
		Event:   "some_slog_event",
		Message: "a backend warning",
	}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	// Also write a mapped event that should NOT appear in debug.
	if err := store.Write(db.Entry{
		Source: db.SourceAgent,
		Level:  db.LevelInfo,
		Event:  "session_start",
	}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategoryDebug})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 debug entry, got %d", len(entries))
	}
	if entries[0].Event != "some_slog_event" {
		t.Errorf("expected event some_slog_event, got %q", entries[0].Event)
	}
	if entries[0].Category != "debug" {
		t.Errorf("expected category debug, got %q", entries[0].Category)
	}
}

func TestGetAuditLog_TerminalConnectedIsSessionCategory(t *testing.T) {
	t.Parallel()
	svc, store := newTestService(t)

	if err := store.Write(db.Entry{
		Source:    db.SourceContainer,
		Level:     db.LevelInfo,
		ProjectID: "proj1",
		Event:     "terminal_connected",
	}); err != nil {
		t.Fatalf("Write() error: %v", err)
	}

	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategorySession})
	if err != nil {
		t.Fatalf("GetAuditLog() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 session entry, got %d", len(entries))
	}
	if entries[0].Event != "terminal_connected" {
		t.Errorf("expected terminal_connected, got %q", entries[0].Event)
	}
	if entries[0].Category != "session" {
		t.Errorf("expected category session, got %q", entries[0].Category)
	}
}

// bytesWriter is a simple io.Writer that appends to a byte slice.
type bytesWriter struct {
	buf *[]byte
}

func (w *bytesWriter) Write(p []byte) (int, error) {
	*w.buf = append(*w.buf, p...)
	return len(p), nil
}

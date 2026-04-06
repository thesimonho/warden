package db

import (
	"testing"
)

// testStandardEvents mirrors the production standard events for testing.
var testStandardEvents = map[string]bool{
	"session_start": true, "session_end": true, "session_exit": true,
	"terminal_connected": true, "terminal_disconnected": true, "stop_failure": true,
	"budget_exceeded": true, "budget_enforcement_failed": true,
	"process_killed": true, "restart_blocked_stale_mounts": true,
	"project_created": true, "project_removed": true,
	"container_created": true, "container_deleted": true, "container_rebuilt": true,
	"cost_reset": true, "audit_purged": true,
}

func TestAuditWriter_OffDropsAll(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditOff, testStandardEvents)

	w.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "session_start"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "tool_use"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries in off mode, got %d", len(entries))
	}
}

func TestAuditWriter_StandardPassesKnownEvents(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditStandard, testStandardEvents)

	w.Write(Entry{Source: SourceContainer, Level: LevelInfo, Event: "session_start"})
	w.Write(Entry{Source: SourceBackend, Level: LevelError, Event: "budget_exceeded"})
	w.Write(Entry{Source: SourceContainer, Level: LevelInfo, Event: "terminal_connected"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries in standard mode for known events, got %d", len(entries))
	}
}

func TestAuditWriter_StandardDropsDetailedEvents(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditStandard, testStandardEvents)

	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "tool_use"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "permission_request"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "user_prompt"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "config_change"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries in standard mode for detailed events, got %d", len(entries))
	}
}

func TestAuditWriter_StandardDropsSlogDebugEvents(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditStandard, testStandardEvents)

	// Slog-generated events have arbitrary snake_case names not in standardEvents.
	w.Write(Entry{Source: SourceBackend, Level: LevelWarn, Event: "container_heartbeat_stale"})
	w.Write(Entry{Source: SourceBackend, Level: LevelError, Event: "failed_to_persist_session_cost"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("expected 0 entries for slog debug events in standard mode, got %d", len(entries))
	}
}

func TestAuditWriter_DetailedPassesAll(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditDetailed, testStandardEvents)

	w.Write(Entry{Source: SourceContainer, Level: LevelInfo, Event: "session_start"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "tool_use"})
	w.Write(Entry{Source: SourceAgent, Level: LevelInfo, Event: "user_prompt"})
	w.Write(Entry{Source: SourceBackend, Level: LevelWarn, Event: "container_heartbeat_stale"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries in detailed mode, got %d", len(entries))
	}
}

func TestAuditWriter_SetModeAtRuntime(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditOff, testStandardEvents)

	w.Write(Entry{Source: SourceContainer, Level: LevelInfo, Event: "session_start"})

	w.SetMode(AuditStandard)
	w.Write(Entry{Source: SourceContainer, Level: LevelInfo, Event: "session_start"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry after mode change, got %d", len(entries))
	}
}

func TestAuditWriter_NilSafe(t *testing.T) {
	var w *AuditWriter
	w.Write(Entry{Event: "test"}) // should not panic
	if w.Mode() != AuditOff {
		t.Fatalf("nil writer mode should be off, got %s", w.Mode())
	}
	w.SetMode(AuditDetailed) // should not panic
}

func TestAuditWriter_FrontendAndExternalEventsPassStandard(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditStandard, testStandardEvents)

	// Frontend and external events always pass in standard mode, regardless
	// of event name. This allows integrators and the web UI to post custom
	// events without needing to register them in the standard allowlist.
	w.Write(Entry{Source: SourceFrontend, Level: LevelInfo, Event: "custom_frontend_event"})
	w.Write(Entry{Source: SourceExternal, Level: LevelInfo, Event: "custom_integrator_event"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries for frontend/external events in standard, got %d", len(entries))
	}
}

func TestAuditWriter_LifecycleEventsPassStandard(t *testing.T) {
	store := newTestStore(t)
	w := NewAuditWriter(store, AuditStandard, testStandardEvents)

	w.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "project_created", ProjectID: "aabbccddee01"})
	w.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "container_created", ProjectID: "aabbccddee01"})
	w.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "cost_reset", ProjectID: "aabbccddee01"})
	w.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "audit_purged", ProjectID: "aabbccddee01"})

	entries, err := store.Read()
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 4 {
		t.Fatalf("expected 4 lifecycle events in standard mode, got %d", len(entries))
	}
}

// newTestStore creates a temporary SQLite store for testing.
func newTestStore(t *testing.T) *Store {
	t.Helper()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("creating test store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

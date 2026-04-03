package db

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestNew_CreatesDirectory(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	subdir := filepath.Join(dir, "logs", "nested")

	logger, err := New(subdir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if _, err := os.Stat(subdir); os.IsNotExist(err) {
		t.Fatal("expected directory to be created")
	}
}

func TestNew_CreatesDatabaseFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	logger, err := New(dir)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	dbPath := filepath.Join(dir, "warden.db")
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("expected database file to be created")
	}
}

func TestWriteAndRead(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	entries := []Entry{
		{Timestamp: ts, Source: SourceAgent, ProjectID: "aabbccddee01", Worktree: "main", Event: "session_start"},
		{Timestamp: ts.Add(time.Second), Source: SourceBackend, Level: LevelInfo, Message: "server started"},
		{Timestamp: ts.Add(2 * time.Second), Source: SourceFrontend, Message: "terminal opened", Attrs: map[string]any{"projectId": "abc"}},
		{Timestamp: ts.Add(3 * time.Second), Source: SourceContainer, ProjectID: "aabbccddee01", Event: "create"},
	}

	for _, e := range entries {
		if err := logger.Write(e); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
	}

	result, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	if len(result) != len(entries) {
		t.Fatalf("expected %d entries, got %d", len(entries), len(result))
	}

	// Results are newest-first (DESC).
	for i, entry := range result {
		want := entries[len(entries)-1-i].Source
		if entry.Source != want {
			t.Errorf("entry[%d].Source = %q, want %q", i, entry.Source, want)
		}
	}
}

func TestWriteAndRead_SortedByTimestamp(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

	// Write out of order.
	if err := logger.Write(Entry{Timestamp: ts.Add(2 * time.Second), Source: SourceBackend, Message: "second"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Timestamp: ts, Source: SourceAgent, Event: "first"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Timestamp: ts.Add(time.Second), Source: SourceFrontend, Message: "middle"}); err != nil {
		t.Fatal(err)
	}

	result, _ := logger.Read()
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}

	// Results are newest-first (DESC).
	if result[0].Message != "second" {
		t.Errorf("expected first entry to be 'second', got %q", result[0].Message)
	}
	if result[1].Message != "middle" {
		t.Errorf("expected second entry to be 'middle', got %q", result[1].Message)
	}
	if result[2].Event != "first" {
		t.Errorf("expected third entry to be 'first', got %q", result[2].Event)
	}
}

func TestClear(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceBackend, Message: "test"}); err != nil {
		t.Fatal(err)
	}

	if err := logger.Clear(); err != nil {
		t.Fatalf("Clear() error: %v", err)
	}

	result, _ := logger.Read()
	if len(result) != 0 {
		t.Errorf("expected 0 entries after clear, got %d", len(result))
	}

	// Can still write after clear.
	if err := logger.Write(Entry{Source: SourceBackend, Message: "after clear"}); err != nil {
		t.Fatal(err)
	}
	result, _ = logger.Read()
	if len(result) != 1 {
		t.Errorf("expected 1 entry after write, got %d", len(result))
	}
}

func TestConcurrentWrites(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	const goroutines = 10
	const entriesPerGoroutine = 50
	var wg sync.WaitGroup

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < entriesPerGoroutine; i++ {
				if err := logger.Write(Entry{
					Source:  SourceBackend,
					Message: "concurrent write",
					Attrs:   map[string]any{"goroutine": id, "index": i},
				}); err != nil {
					t.Errorf("Write error: %v", err)
				}
			}
		}(g)
	}

	wg.Wait()

	result, err := logger.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}

	expected := goroutines * entriesPerGoroutine
	if len(result) != expected {
		t.Errorf("expected %d entries, got %d", expected, len(result))
	}
}

func TestNilLogger(t *testing.T) {
	t.Parallel()
	var logger *Store

	if err := logger.Write(Entry{Source: SourceBackend, Message: "test"}); err != nil {
		t.Errorf("nil Write() should not error, got %v", err)
	}

	entries, err := logger.Read()
	if err != nil {
		t.Errorf("nil Read() should not error, got %v", err)
	}
	if entries != nil {
		t.Errorf("nil Read() should return nil, got %v", entries)
	}

	entries, err = logger.Query(QueryFilters{Source: SourceAgent})
	if err != nil {
		t.Errorf("nil Query() should not error, got %v", err)
	}
	if entries != nil {
		t.Errorf("nil Query() should return nil, got %v", entries)
	}

	ids, err := logger.DistinctProjectIDs()
	if err != nil {
		t.Errorf("nil DistinctProjectIDs() should not error, got %v", err)
	}
	if ids != nil {
		t.Errorf("nil DistinctProjectIDs() should return nil, got %v", ids)
	}

	if err := logger.Clear(); err != nil {
		t.Errorf("nil Clear() should not error, got %v", err)
	}

	if err := logger.Close(); err != nil {
		t.Errorf("nil Close() should not error, got %v", err)
	}
}

func TestEntryJSONFormat(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ts := time.Date(2026, 3, 19, 12, 0, 0, 0, time.UTC)
	if err := logger.Write(Entry{
		Timestamp:     ts,
		Source:        SourceAgent,
		ProjectID:     "aabbccddee01",
		ContainerName: "my-project",
		Worktree:      "feature-x",
		Event:         "session_start",
		Data:          json.RawMessage(`{"session_id":"abc"}`),
	}); err != nil {
		t.Fatal(err)
	}

	result, _ := logger.Read()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	entry := result[0]
	if entry.Source != SourceAgent {
		t.Errorf("Source = %q, want %q", entry.Source, SourceAgent)
	}
	if entry.ProjectID != "aabbccddee01" {
		t.Errorf("ProjectID = %q, want %q", entry.ProjectID, "aabbccddee01")
	}
	if entry.ContainerName != "my-project" {
		t.Errorf("ContainerName = %q, want %q", entry.ContainerName, "my-project")
	}
	if entry.Worktree != "feature-x" {
		t.Errorf("Worktree = %q, want %q", entry.Worktree, "feature-x")
	}
	if entry.Event != "session_start" {
		t.Errorf("Event = %q, want %q", entry.Event, "session_start")
	}
	if string(entry.Data) != `{"session_id":"abc"}` {
		t.Errorf("Data = %s, want %s", entry.Data, `{"session_id":"abc"}`)
	}
}

func TestWrite_AutoTimestamp(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	before := time.Now().UTC()
	if err := logger.Write(Entry{Source: SourceBackend, Message: "auto ts"}); err != nil {
		t.Fatal(err)
	}
	after := time.Now().UTC()

	result, _ := logger.Read()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	ts := result[0].Timestamp
	if ts.Before(before.Truncate(time.Microsecond)) || ts.After(after.Add(time.Microsecond)) {
		t.Errorf("auto timestamp %v not between %v and %v", ts, before, after)
	}
}

func TestQuery_FilterBySource(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceAgent, Event: "agent_event"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Event: "backend_event"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, Event: "agent_event_2"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{Source: SourceAgent})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 agent entries, got %d", len(result))
	}
	for _, e := range result {
		if e.Source != SourceAgent {
			t.Errorf("expected source agent, got %q", e.Source)
		}
	}
}

func TestQuery_FilterByLevel(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceBackend, Level: LevelInfo, Event: "info_event"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Level: LevelWarn, Event: "warn_event"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Level: LevelError, Event: "error_event"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{Level: LevelWarn})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 warn entry, got %d", len(result))
	}
	if result[0].Level != LevelWarn {
		t.Errorf("expected level warn, got %q", result[0].Level)
	}
}

func TestQuery_FilterByProjectID(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "aabbccddee01", Event: "e1"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "112233445566", Event: "e2"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "aabbccddee01", Event: "e3"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries for project aabbccddee01, got %d", len(result))
	}
}

func TestQuery_FilterBySince(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := logger.Write(Entry{Timestamp: ts, Source: SourceAgent, Event: "old"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Timestamp: ts.Add(time.Hour), Source: SourceAgent, Event: "new"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{Since: ts.Add(30 * time.Minute)})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry after since, got %d", len(result))
	}
	if result[0].Event != "new" {
		t.Errorf("expected 'new' event, got %q", result[0].Event)
	}
}

func TestQuery_LimitAndOffset(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ts := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	for i := range 10 {
		if err := logger.Write(Entry{
			Timestamp: ts.Add(time.Duration(i) * time.Second),
			Source:    SourceAgent,
			Event:     fmt.Sprintf("event_%d", i),
		}); err != nil {
			t.Fatal(err)
		}
	}

	// Limit to 3 — newest first.
	result, err := logger.Query(QueryFilters{Limit: 3})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(result))
	}
	if result[0].Event != "event_9" {
		t.Errorf("expected event_9, got %q", result[0].Event)
	}

	// Offset 3, limit 2 — skips 3 newest, takes next 2.
	result, err = logger.Query(QueryFilters{Limit: 2, Offset: 3})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
	if result[0].Event != "event_6" {
		t.Errorf("expected event_6, got %q", result[0].Event)
	}
}

func TestQuery_CombinedFilters(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceAgent, Level: LevelInfo, ProjectID: "aabbccddee01", Event: "e1"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, Level: LevelWarn, ProjectID: "aabbccddee01", Event: "e2"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Level: LevelWarn, ProjectID: "aabbccddee01", Event: "e3"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, Level: LevelWarn, ProjectID: "112233445566", Event: "e4"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{
		Source:    SourceAgent,
		Level:     LevelWarn,
		ProjectID: "aabbccddee01",
	})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry matching all filters, got %d", len(result))
	}
	if result[0].Event != "e2" {
		t.Errorf("expected event e2, got %q", result[0].Event)
	}
}

func TestDistinctProjectIDs(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "112233445566", Event: "e1"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "aabbccddee01", Event: "e2"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, ProjectID: "112233445566", Event: "e3"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Event: "no-project"}); err != nil { // empty project ID
		t.Fatal(err)
	}

	ids, err := logger.DistinctProjectIDs()
	if err != nil {
		t.Fatalf("DistinctProjectIDs() error: %v", err)
	}

	if len(ids) != 2 {
		t.Fatalf("expected 2 distinct project IDs, got %d: %v", len(ids), ids)
	}
	// Should be sorted.
	if ids[0] != "112233445566" || ids[1] != "aabbccddee01" {
		t.Errorf("expected [112233445566, aabbccddee01], got %v", ids)
	}
}

func TestDistinctProjectIDs_Empty(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	ids, err := logger.DistinctProjectIDs()
	if err != nil {
		t.Fatalf("DistinctProjectIDs() error: %v", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected empty list, got %v", ids)
	}
}

func TestAttrsRoundTrip(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	attrs := map[string]any{
		"count":   float64(42),
		"enabled": true,
		"name":    "test",
	}
	if err := logger.Write(Entry{Source: SourceBackend, Event: "test", Attrs: attrs}); err != nil {
		t.Fatal(err)
	}

	result, _ := logger.Read()
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	got := result[0].Attrs
	if got["count"] != float64(42) {
		t.Errorf("attrs.count = %v, want 42", got["count"])
	}
	if got["enabled"] != true {
		t.Errorf("attrs.enabled = %v, want true", got["enabled"])
	}
	if got["name"] != "test" {
		t.Errorf("attrs.name = %v, want 'test'", got["name"])
	}
}

// --- Project tests ---

func TestInsertAndGetProject(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	p := ProjectRow{
		ProjectID:       "aabbccddee01",
		Name:            "my-project",
		HostPath:        "/home/user/code",
		Image:           "warden:latest",
		EnvVars:         json.RawMessage(`{"FOO":"bar"}`),
		Mounts:          json.RawMessage(`[{"hostPath":"/a","containerPath":"/b","readOnly":false}]`),
		SkipPermissions: true,
		NetworkMode:     "restricted",
		AllowedDomains:  "github.com,npmjs.org",
		ContainerID:     "abc123def456",
		ContainerName:   "my-project",
	}
	if err := store.InsertProject(p); err != nil {
		t.Fatalf("InsertProject() error: %v", err)
	}

	got, err := store.GetProject("aabbccddee01", "claude-code")
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if got == nil {
		t.Fatal("GetProject() returned nil")
		return // unreachable but helps staticcheck
	}
	if got.ProjectID != "aabbccddee01" {
		t.Errorf("ProjectID = %q, want %q", got.ProjectID, "aabbccddee01")
	}
	if got.Name != "my-project" {
		t.Errorf("Name = %q, want %q", got.Name, "my-project")
	}
	if got.HostPath != "/home/user/code" {
		t.Errorf("HostPath = %q, want %q", got.HostPath, "/home/user/code")
	}
	if got.Image != "warden:latest" {
		t.Errorf("Image = %q, want %q", got.Image, "warden:latest")
	}
	if string(got.EnvVars) != `{"FOO":"bar"}` {
		t.Errorf("EnvVars = %s, want %s", got.EnvVars, `{"FOO":"bar"}`)
	}
	if !got.SkipPermissions {
		t.Error("SkipPermissions should be true")
	}
	if got.NetworkMode != "restricted" {
		t.Errorf("NetworkMode = %q, want %q", got.NetworkMode, "restricted")
	}
	if got.AllowedDomains != "github.com,npmjs.org" {
		t.Errorf("AllowedDomains = %q, want %q", got.AllowedDomains, "github.com,npmjs.org")
	}
	if got.ContainerID != "abc123def456" {
		t.Errorf("ContainerID = %q, want %q", got.ContainerID, "abc123def456")
	}
	if got.ContainerName != "my-project" {
		t.Errorf("ContainerName = %q, want %q", got.ContainerName, "my-project")
	}
}

func TestUpdateProjectSettings(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	// Insert a project first.
	p := ProjectRow{
		ProjectID:       "aabbccddee02",
		Name:            "original-name",
		HostPath:        "/home/user/code",
		Image:           "warden:latest",
		EnvVars:         json.RawMessage(`{"FOO":"bar"}`),
		SkipPermissions: false,
		CostBudget:      0,
		ContainerID:     "abc123",
		ContainerName:   "original-name",
	}
	if err := store.InsertProject(p); err != nil {
		t.Fatalf("InsertProject() error: %v", err)
	}

	// Update only lightweight settings.
	if err := store.UpdateProjectSettings("aabbccddee02", "claude-code", "new-name", "new-name", true, 42.5); err != nil {
		t.Fatalf("UpdateProjectSettings() error: %v", err)
	}

	got, err := store.GetProject("aabbccddee02", "claude-code")
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if got.Name != "new-name" {
		t.Errorf("Name = %q, want %q", got.Name, "new-name")
	}
	if got.ContainerName != "new-name" {
		t.Errorf("ContainerName = %q, want %q", got.ContainerName, "new-name")
	}
	if !got.SkipPermissions {
		t.Error("SkipPermissions should be true")
	}
	if got.CostBudget != 42.5 {
		t.Errorf("CostBudget = %f, want 42.5", got.CostBudget)
	}
	// Verify non-updated fields are preserved.
	if got.Image != "warden:latest" {
		t.Errorf("Image = %q, want %q (should be unchanged)", got.Image, "warden:latest")
	}
	if string(got.EnvVars) != `{"FOO":"bar"}` {
		t.Errorf("EnvVars = %s, want %s (should be unchanged)", got.EnvVars, `{"FOO":"bar"}`)
	}
	if got.ContainerID != "abc123" {
		t.Errorf("ContainerID = %q, want %q (should be unchanged)", got.ContainerID, "abc123")
	}
}

func TestGetProject_NotFound(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	got, err := store.GetProject("nonexistent", "claude-code")
	if err != nil {
		t.Fatalf("GetProject() error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nonexistent project, got %v", got)
	}
}

func TestGetProjectsByPath(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{
		ProjectID: "aabbccddee01",
		Name:      "my-project",
		HostPath:  "/home/user/code",
	}); err != nil {
		t.Fatal(err)
	}

	got, err := store.GetProjectsByPath("/home/user/code")
	if err != nil {
		t.Fatalf("GetProjectsByPath() error: %v", err)
	}
	if len(got) == 0 {
		t.Fatal("GetProjectsByPath() returned empty slice")
	}
	if got[0].ProjectID != "aabbccddee01" {
		t.Errorf("ProjectID = %q, want %q", got[0].ProjectID, "aabbccddee01")
	}

	got, err = store.GetProjectsByPath("/nonexistent")
	if err != nil {
		t.Fatalf("GetProjectsByPath() error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty slice for nonexistent path, got %v", got)
	}
}

func TestListProjectKeys(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{ProjectID: "112233445566", Name: "proj-b", HostPath: "/b"}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "proj-a", HostPath: "/a"}); err != nil {
		t.Fatal(err)
	}

	keys, err := store.ListProjectKeys()
	if err != nil {
		t.Fatalf("ListProjectKeys() error: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("expected 2 keys, got %d", len(keys))
	}
	// Ordered by added_at (insertion order).
	if keys[0].ProjectID != "112233445566" || keys[1].ProjectID != "aabbccddee01" {
		t.Errorf("expected [112233445566, aabbccddee01], got %v", keys)
	}
}

func TestHasProject(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "exists", HostPath: "/a"}); err != nil {
		t.Fatal(err)
	}

	has, err := store.HasProject("aabbccddee01", "claude-code")
	if err != nil {
		t.Fatalf("HasProject() error: %v", err)
	}
	if !has {
		t.Error("expected HasProject to return true")
	}

	has, err = store.HasProject("missing", "claude-code")
	if err != nil {
		t.Fatalf("HasProject() error: %v", err)
	}
	if has {
		t.Error("expected HasProject to return false for missing project")
	}
}

func TestDeleteProject(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "to-delete", HostPath: "/a"}); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteProject("aabbccddee01", "claude-code"); err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}

	got, _ := store.GetProject("aabbccddee01", "claude-code")
	if got != nil {
		t.Error("expected project to be deleted")
	}
}

func TestDeleteProject_RetainsAuditEvents(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	// Insert project and some events for it.
	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "proj-a", HostPath: "/a"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(Entry{Source: SourceAgent, ProjectID: "aabbccddee01", Event: "session_start"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Write(Entry{Source: SourceAgent, ProjectID: "aabbccddee01", Event: "session_end"}); err != nil {
		t.Fatal(err)
	}

	// Delete the project.
	if err := store.DeleteProject("aabbccddee01", "claude-code"); err != nil {
		t.Fatalf("DeleteProject() error: %v", err)
	}

	// Project row should be gone.
	keys, _ := store.ListProjectKeys()
	for _, key := range keys {
		if key.ProjectID == "aabbccddee01" {
			t.Error("expected project to be removed from projects table")
		}
	}

	// Audit events should still exist for historical analysis.
	entries, _ := store.Query(QueryFilters{ProjectID: "aabbccddee01"})
	if len(entries) != 2 {
		t.Errorf("expected 2 audit events retained after project delete, got %d", len(entries))
	}
}

func TestInsertProject_Upsert(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "proj", HostPath: "/a", Image: "old-image"}); err != nil {
		t.Fatal(err)
	}
	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "proj", HostPath: "/a", Image: "new-image"}); err != nil {
		t.Fatal(err)
	}

	got, _ := store.GetProject("aabbccddee01", "claude-code")
	if got.Image != "new-image" {
		t.Errorf("Image = %q, want %q after upsert", got.Image, "new-image")
	}

	keys, _ := store.ListProjectKeys()
	if len(keys) != 1 {
		t.Errorf("expected 1 project after upsert, got %d", len(keys))
	}
}

func TestUpdateProjectContainer(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "proj", HostPath: "/a"}); err != nil {
		t.Fatal(err)
	}

	if err := store.UpdateProjectContainer("aabbccddee01", "claude-code", "docker-id-123", "my-container"); err != nil {
		t.Fatalf("UpdateProjectContainer() error: %v", err)
	}

	got, _ := store.GetProject("aabbccddee01", "claude-code")
	if got.ContainerID != "docker-id-123" {
		t.Errorf("ContainerID = %q, want %q", got.ContainerID, "docker-id-123")
	}
	if got.ContainerName != "my-container" {
		t.Errorf("ContainerName = %q, want %q", got.ContainerName, "my-container")
	}

	// Clear container.
	if err := store.UpdateProjectContainer("aabbccddee01", "claude-code", "", ""); err != nil {
		t.Fatal(err)
	}
	got, _ = store.GetProject("aabbccddee01", "claude-code")
	if got.ContainerID != "" {
		t.Errorf("expected empty ContainerID after clear, got %q", got.ContainerID)
	}
}

// --- Session cost tests ---

func TestUpsertSessionCost(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	if err := store.UpsertSessionCost("aabbccddee01", "claude-code", "sess1", 1.50, false); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertSessionCost("aabbccddee01", "claude-code", "sess2", 0.50, false); err != nil {
		t.Fatal(err)
	}

	cost, err := store.GetProjectTotalCost("aabbccddee01", "claude-code")
	if err != nil {
		t.Fatal(err)
	}
	if cost.TotalCost != 2.0 {
		t.Errorf("TotalCost = %v, want 2.0", cost.TotalCost)
	}

	// Upsert with higher cost — should update.
	if err := store.UpsertSessionCost("aabbccddee01", "claude-code", "sess1", 3.00, false); err != nil {
		t.Fatal(err)
	}
	cost, _ = store.GetProjectTotalCost("aabbccddee01", "claude-code")
	if cost.TotalCost != 3.5 {
		t.Errorf("TotalCost = %v, want 3.5 after upsert", cost.TotalCost)
	}
}

func TestGetAllProjectTotalCosts(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	_ = store.UpsertSessionCost("aabbccddee01", "claude-code", "s1", 1.0, false)
	_ = store.UpsertSessionCost("112233445566", "claude-code", "s2", 2.0, true)

	costs, err := store.GetAllProjectTotalCosts()
	if err != nil {
		t.Fatal(err)
	}
	if len(costs) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(costs))
	}
	key1 := ProjectAgentKey{ProjectID: "aabbccddee01", AgentType: "claude-code"}
	key2 := ProjectAgentKey{ProjectID: "112233445566", AgentType: "claude-code"}
	if costs[key1].TotalCost != 1.0 {
		t.Errorf("proj1 cost = %v, want 1.0", costs[key1].TotalCost)
	}
	if costs[key2].TotalCost != 2.0 {
		t.Errorf("proj2 cost = %v, want 2.0", costs[key2].TotalCost)
	}
}

func TestDeleteProjectCosts(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	_ = store.UpsertSessionCost("aabbccddee01", "claude-code", "s1", 5.0, false)
	_ = store.UpsertSessionCost("aabbccddee01", "claude-code", "s2", 3.0, false)

	if err := store.DeleteProjectCosts("aabbccddee01", "claude-code"); err != nil {
		t.Fatal(err)
	}

	cost, _ := store.GetProjectTotalCost("aabbccddee01", "claude-code")
	if cost.TotalCost != 0 {
		t.Errorf("expected 0 cost after delete, got %v", cost.TotalCost)
	}
}

// --- Settings tests ---

func TestGetSetSetting(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	// Default when missing.
	val := store.GetSetting("runtime", "docker")
	if val != "docker" {
		t.Errorf("expected default 'docker', got %q", val)
	}

	// Set and read back.
	if err := store.SetSetting("runtime", "custom-value"); err != nil {
		t.Fatalf("SetSetting() error: %v", err)
	}
	val = store.GetSetting("runtime", "docker")
	if val != "custom-value" {
		t.Errorf("expected 'custom-value', got %q", val)
	}

	// Upsert.
	if err := store.SetSetting("runtime", "docker"); err != nil {
		t.Fatal(err)
	}
	val = store.GetSetting("runtime", "")
	if val != "docker" {
		t.Errorf("expected 'docker' after upsert, got %q", val)
	}
}

func TestQuery_ExcludeEvents(t *testing.T) {
	t.Parallel()
	logger, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer logger.Close() //nolint:errcheck

	if err := logger.Write(Entry{Source: SourceAgent, Event: "session_start"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, Event: "tool_use"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceAgent, Event: "user_prompt"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Event: "some_slog_warning"}); err != nil {
		t.Fatal(err)
	}
	if err := logger.Write(Entry{Source: SourceBackend, Event: "container_heartbeat_stale"}); err != nil {
		t.Fatal(err)
	}

	result, err := logger.Query(QueryFilters{
		ExcludeEvents: []string{"session_start", "tool_use", "user_prompt"},
	})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries after excluding 3, got %d", len(result))
	}

	excluded := map[string]bool{"session_start": true, "tool_use": true, "user_prompt": true}
	for _, e := range result {
		if excluded[e.Event] {
			t.Errorf("excluded event %q should not appear in results", e.Event)
		}
	}
}

func TestNilStore_ProjectsAndSettings(t *testing.T) {
	t.Parallel()
	var store *Store

	if err := store.InsertProject(ProjectRow{ProjectID: "aabbccddee01", Name: "test", HostPath: "/a"}); err != nil {
		t.Errorf("nil InsertProject() should not error, got %v", err)
	}
	if err := store.DeleteProject("aabbccddee01", "claude-code"); err != nil {
		t.Errorf("nil DeleteProject() should not error, got %v", err)
	}

	keys, err := store.ListProjectKeys()
	if err != nil || keys != nil {
		t.Errorf("nil ListProjectKeys() = (%v, %v), want (nil, nil)", keys, err)
	}

	got, err := store.GetProject("aabbccddee01", "claude-code")
	if err != nil || got != nil {
		t.Errorf("nil GetProject() = (%v, %v), want (nil, nil)", got, err)
	}

	has, err := store.HasProject("aabbccddee01", "claude-code")
	if err != nil || has {
		t.Errorf("nil HasProject() = (%v, %v), want (false, nil)", has, err)
	}

	val := store.GetSetting("key", "default")
	if val != "default" {
		t.Errorf("nil GetSetting() = %q, want 'default'", val)
	}

	if err := store.SetSetting("key", "val"); err != nil {
		t.Errorf("nil SetSetting() should not error, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// SourceID dedup (INSERT OR IGNORE on compound unique index)
// ---------------------------------------------------------------------------

func TestWrite_DuplicateSourceID_SameProject_OnlyOneRow(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	entry := Entry{
		Source:    SourceAgent,
		Level:     LevelInfo,
		ProjectID: "aabbccddee01",
		Event:     "tool_use",
		Message:   "first write",
		SourceID:  "deadbeef12345678",
	}

	if err := store.Write(entry); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}

	// Second write with the same SourceID and ProjectID should be silently ignored.
	entry.Message = "second write"
	if err := store.Write(entry); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}

	result, err := store.Query(QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry (dedup), got %d", len(result))
	}
	if result[0].Message != "first write" {
		t.Errorf("expected first write to win, got %q", result[0].Message)
	}
}

func TestWrite_EmptySourceID_BothInserted(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	entry := Entry{
		Source:    SourceAgent,
		Level:     LevelInfo,
		ProjectID: "aabbccddee01",
		Event:     "tool_use",
		Message:   "no source id",
		// SourceID is empty — becomes NULL in SQLite.
	}

	if err := store.Write(entry); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}
	if err := store.Write(entry); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}

	result, err := store.Query(QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (NULL source_ids don't trigger uniqueness), got %d", len(result))
	}
}

func TestWrite_DifferentSourceIDs_BothInserted(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	base := Entry{
		Source:    SourceAgent,
		Level:     LevelInfo,
		ProjectID: "aabbccddee01",
		Event:     "tool_use",
	}

	first := base
	first.SourceID = "aaaa000011112222"
	first.Message = "first"

	second := base
	second.SourceID = "bbbb333344445555"
	second.Message = "second"

	if err := store.Write(first); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}
	if err := store.Write(second); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}

	result, err := store.Query(QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (different SourceIDs), got %d", len(result))
	}
}

func TestWrite_SameSourceID_DifferentProjects_BothInserted(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	sourceID := "deadbeef12345678"

	first := Entry{
		Source:    SourceAgent,
		Level:     LevelInfo,
		ProjectID: "aabbccddee01",
		Event:     "tool_use",
		Message:   "project A",
		SourceID:  sourceID,
	}

	second := Entry{
		Source:    SourceAgent,
		Level:     LevelInfo,
		ProjectID: "112233445566",
		Event:     "tool_use",
		Message:   "project B",
		SourceID:  sourceID,
	}

	if err := store.Write(first); err != nil {
		t.Fatalf("first Write() error: %v", err)
	}
	if err := store.Write(second); err != nil {
		t.Fatalf("second Write() error: %v", err)
	}

	// Both should exist because the compound key is (project_id, source_id).
	all, err := store.Read()
	if err != nil {
		t.Fatalf("Read() error: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("expected 2 entries (same SourceID, different projects), got %d", len(all))
	}
}

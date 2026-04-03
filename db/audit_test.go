package db

import (
	"testing"
	"time"
)

func TestQuery_FilterByEvents(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	now := time.Now().UTC()
	events := []Entry{
		{Timestamp: now, Source: SourceAgent, Level: LevelInfo, Event: "session_start", ProjectID: "aabbccddee01"},
		{Timestamp: now.Add(time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Message: "Read"},
		{Timestamp: now.Add(2 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "user_prompt", ProjectID: "aabbccddee01"},
		{Timestamp: now.Add(4 * time.Second), Source: SourceBackend, Level: LevelInfo, Event: "container_create", ProjectID: "aabbccddee01"},
	}
	for _, e := range events {
		if writeErr := store.Write(e); writeErr != nil {
			t.Fatalf("Write() error: %v", writeErr)
		}
	}

	entries, err := store.Query(QueryFilters{
		Events: []string{"tool_use"},
	})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Event != "tool_use" {
		t.Errorf("expected tool_use, got %s", entries[0].Event)
	}
}

func TestQuery_FilterByWorktree(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	now := time.Now().UTC()
	events := []Entry{
		{Timestamp: now, Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Worktree: "main"},
		{Timestamp: now.Add(time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Worktree: "feat-1"},
		{Timestamp: now.Add(2 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Worktree: "main"},
	}
	for _, e := range events {
		if writeErr := store.Write(e); writeErr != nil {
			t.Fatalf("Write() error: %v", writeErr)
		}
	}

	entries, err := store.Query(QueryFilters{Worktree: "feat-1"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}

	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Worktree != "feat-1" {
		t.Errorf("expected feat-1, got %s", entries[0].Worktree)
	}
}

func TestQuery_FilterByUntil(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	events := []Entry{
		{Timestamp: base, Source: SourceAgent, Level: LevelInfo, Event: "tool_use"},
		{Timestamp: base.Add(time.Hour), Source: SourceAgent, Level: LevelInfo, Event: "tool_use"},
		{Timestamp: base.Add(2 * time.Hour), Source: SourceAgent, Level: LevelInfo, Event: "tool_use"},
	}
	for _, e := range events {
		if writeErr := store.Write(e); writeErr != nil {
			t.Fatalf("Write() error: %v", writeErr)
		}
	}

	entries, err := store.Query(QueryFilters{Until: base.Add(90 * time.Minute)})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}

	if len(entries) != 2 {
		t.Fatalf("expected 2 entries (before 1.5h), got %d", len(entries))
	}
}

func TestQueryAuditSummary(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	now := time.Now().UTC()
	events := []Entry{
		{Timestamp: now, Source: SourceAgent, Level: LevelInfo, Event: "session_start", ProjectID: "aabbccddee01", Worktree: "main"},
		{Timestamp: now.Add(time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Worktree: "main", Message: "Read"},
		{Timestamp: now.Add(2 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", ProjectID: "aabbccddee01", Worktree: "feat", Message: "Edit"},
		{Timestamp: now.Add(3 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "user_prompt", ProjectID: "112233445566", Worktree: "main"},
	}
	for _, e := range events {
		if writeErr := store.Write(e); writeErr != nil {
			t.Fatalf("Write() error: %v", writeErr)
		}
	}

	summary, err := store.QueryAuditSummary(QueryFilters{})
	if err != nil {
		t.Fatalf("QueryAuditSummary() error: %v", err)
	}

	if summary.TotalSessions != 1 {
		t.Errorf("expected 1 session, got %d", summary.TotalSessions)
	}
	if summary.TotalToolUses != 2 {
		t.Errorf("expected 2 tool uses, got %d", summary.TotalToolUses)
	}
	if summary.TotalPrompts != 1 {
		t.Errorf("expected 1 prompt, got %d", summary.TotalPrompts)
	}
	if summary.UniqueProjects != 2 {
		t.Errorf("expected 2 unique projects, got %d", summary.UniqueProjects)
	}
	if summary.UniqueWorktrees != 2 {
		t.Errorf("expected 2 unique worktrees, got %d", summary.UniqueWorktrees)
	}
}

func TestQueryTopTools(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	now := time.Now().UTC()
	events := []Entry{
		{Timestamp: now, Source: SourceAgent, Level: LevelInfo, Event: "tool_use", Message: "Read"},
		{Timestamp: now.Add(time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", Message: "Read"},
		{Timestamp: now.Add(2 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", Message: "Edit"},
		{Timestamp: now.Add(3 * time.Second), Source: SourceAgent, Level: LevelInfo, Event: "tool_use", Message: "Write"},
	}
	for _, e := range events {
		if writeErr := store.Write(e); writeErr != nil {
			t.Fatalf("Write() error: %v", writeErr)
		}
	}

	tools, err := store.QueryTopTools(QueryFilters{}, 2)
	if err != nil {
		t.Fatalf("QueryTopTools() error: %v", err)
	}

	if len(tools) != 2 {
		t.Fatalf("expected 2 top tools (limit), got %d", len(tools))
	}
	if tools[0].Name != "Read" {
		t.Errorf("expected top tool to be Read, got %s", tools[0].Name)
	}
	if tools[0].Count != 2 {
		t.Errorf("expected Read count 2, got %d", tools[0].Count)
	}
}

func TestQueryAuditSummary_NilStore(t *testing.T) {
	t.Parallel()
	var store *Store

	summary, err := store.QueryAuditSummary(QueryFilters{})
	if err != nil {
		t.Fatalf("QueryAuditSummary() error: %v", err)
	}
	if summary.TotalSessions != 0 {
		t.Errorf("expected 0 sessions, got %d", summary.TotalSessions)
	}
}

func TestGetCostInTimeRange(t *testing.T) {
	t.Parallel()
	store, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer store.Close() //nolint:errcheck

	// Insert sessions with known timestamps by manipulating the DB directly.
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions := []struct {
		projectID string
		sessionID string
		cost      float64
		createdAt time.Time
		updatedAt time.Time
	}{
		{"aabbccddee01", "s1", 1.00, base, base.Add(time.Hour)},                        // 00:00 – 01:00
		{"aabbccddee01", "s2", 2.50, base.Add(2 * time.Hour), base.Add(3 * time.Hour)}, // 02:00 – 03:00
		{"112233445566", "s3", 4.00, base.Add(time.Hour), base.Add(4 * time.Hour)},     // 01:00 – 04:00
	}

	for _, s := range sessions {
		_, insertErr := store.db.Exec(
			`INSERT INTO session_costs (project_id, agent_type, session_id, cost, is_estimated, created_at, updated_at)
			 VALUES (?, 'claude-code', ?, ?, 0, ?, ?)`,
			s.projectID, s.sessionID, s.cost,
			s.createdAt.Format(time.RFC3339Nano),
			s.updatedAt.Format(time.RFC3339Nano),
		)
		if insertErr != nil {
			t.Fatalf("inserting test session: %v", insertErr)
		}
	}

	tests := []struct {
		name      string
		projectID string
		since     time.Time
		until     time.Time
		wantCost  float64
	}{
		{
			name:     "no filters returns all",
			wantCost: 7.50,
		},
		{
			name:      "filter by project ID",
			projectID: "aabbccddee01",
			wantCost:  3.50,
		},
		{
			name:     "since filters out sessions updated before",
			since:    base.Add(90 * time.Minute), // s1 updated at 01:00, excluded
			wantCost: 6.50,                       // s2 (2.50) + s3 (4.00)
		},
		{
			name:     "until filters out sessions created after",
			until:    base.Add(90 * time.Minute), // s2 created at 02:00, excluded
			wantCost: 5.00,                       // s1 (1.00) + s3 (4.00)
		},
		{
			name:     "since and until narrow to overlapping sessions",
			since:    base.Add(90 * time.Minute),  // s1 out (updated 01:00)
			until:    base.Add(150 * time.Minute), // s2 in (created 02:00), s3 in (created 01:00)
			wantCost: 6.50,
		},
		{
			name:     "range with no overlap returns zero",
			since:    base.Add(5 * time.Hour),
			until:    base.Add(6 * time.Hour),
			wantCost: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			row, queryErr := store.GetCostInTimeRange(tt.projectID, tt.since, tt.until)
			if queryErr != nil {
				t.Fatalf("GetCostInTimeRange() error: %v", queryErr)
			}
			if row.TotalCost != tt.wantCost {
				t.Errorf("TotalCost = %.2f, want %.2f", row.TotalCost, tt.wantCost)
			}
		})
	}
}

func TestGetCostInTimeRange_NilStore(t *testing.T) {
	t.Parallel()
	var store *Store

	row, err := store.GetCostInTimeRange("", time.Time{}, time.Time{})
	if err != nil {
		t.Fatalf("GetCostInTimeRange() error: %v", err)
	}
	if row.TotalCost != 0 {
		t.Errorf("expected 0 cost, got %.2f", row.TotalCost)
	}
}

func TestQueryTopTools_NilStore(t *testing.T) {
	t.Parallel()
	var store *Store

	tools, err := store.QueryTopTools(QueryFilters{}, 10)
	if err != nil {
		t.Fatalf("QueryTopTools() error: %v", err)
	}
	if tools != nil {
		t.Errorf("expected nil, got %v", tools)
	}
}

package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
)

func TestListProjects(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "test-project", State: "running"},
		},
	}
	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "test-project", Name: "test-project", HostPath: "/test/test-project"})
	svc := New(ServiceDeps{Engine: mock, DB: database})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", projects[0].Name)
	}
}

func TestListProjects_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{projectsErr: errors.New("docker down")}
	database := testDB(t)
	svc := New(ServiceDeps{Engine: mock, DB: database})

	_, err := svc.ListProjects(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListProjects_OverlaysCost(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running", TotalCost: 0},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})
	_ = database.UpsertSessionCost("my-project", "session-abc", 1.5, false)

	svc := New(ServiceDeps{Engine: mock, DB: database})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if projects[0].TotalCost != 1.5 {
		t.Errorf("expected cost 1.5, got %f", projects[0].TotalCost)
	}
}

func TestListProjects_OverlaysCostIsEstimated(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running"},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})
	_ = database.UpsertSessionCost("my-project", "session-abc", 2.75, true)

	svc := New(ServiceDeps{Engine: mock, DB: database})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !projects[0].IsEstimatedCost {
		t.Error("expected IsEstimatedCost to be true for subscription user")
	}
}

func TestListProjects_OverlaysCostIsEstimatedFromDB(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running"},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})
	_ = database.UpsertSessionCost("my-project", "session-abc", 3.50, true)

	svc := New(ServiceDeps{Engine: mock, DB: database})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if projects[0].TotalCost != 3.50 {
		t.Errorf("expected cost 3.50, got %f", projects[0].TotalCost)
	}
	if !projects[0].IsEstimatedCost {
		t.Error("expected IsEstimatedCost to be true from DB")
	}
}

func TestAddProject(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: database})

	result, err := svc.AddProject("my-project", "/home/user/my-project")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ProjectID == "" {
		t.Error("expected non-empty ProjectID")
	}

	has, _ := database.HasProject(result.ProjectID)
	if !has {
		t.Error("expected project to be added to database")
	}
}

func TestRemoveProject(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: database})

	if _, err := svc.RemoveProject("my-project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	has, _ := database.HasProject("my-project")
	if has {
		t.Error("expected project to be removed from database")
	}
}

func TestRemoveProject_AuditOff_CleansCostsAndEvents(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/tmp/my-project"})
	_ = database.UpsertSessionCost("my-project", "sess-1", 5.00, false)
	_ = database.Write(db.Entry{ProjectID: "my-project", Event: "session_start"})

	// Default audit mode is "off".
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: database})

	if _, err := svc.RemoveProject("my-project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cost, _ := database.GetProjectTotalCost("my-project")
	if cost.TotalCost != 0 {
		t.Errorf("expected costs cleaned up when audit off, got %f", cost.TotalCost)
	}

	events, _ := database.Query(db.QueryFilters{ProjectID: "my-project"})
	if len(events) != 0 {
		t.Errorf("expected events cleaned up when audit off, got %d", len(events))
	}
}

func TestRemoveProject_AuditOn_PreservesCostsAndEvents(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/tmp/my-project"})
	_ = database.UpsertSessionCost("my-project", "sess-1", 5.00, false)
	_ = database.Write(db.Entry{ProjectID: "my-project", Event: "session_start"})
	_ = database.SetSetting("auditLogMode", string(api.AuditLogStandard))

	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: database})

	if _, err := svc.RemoveProject("my-project"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cost, _ := database.GetProjectTotalCost("my-project")
	if cost.TotalCost != 5.00 {
		t.Errorf("expected costs preserved when audit on, got %f", cost.TotalCost)
	}

	events, _ := database.Query(db.QueryFilters{ProjectID: "my-project"})
	if len(events) == 0 {
		t.Error("expected events preserved when audit on")
	}
}

func TestStopProject(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}
	database := testDB(t)
	svc := New(ServiceDeps{Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	if _, err := svc.StopProject(context.Background(), row.ProjectID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopProject_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{stopErr: errors.New("not found")}
	database := testDB(t)
	svc := New(ServiceDeps{Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	_, err := svc.StopProject(context.Background(), row.ProjectID)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListProjects_OverlaysAttention(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running"},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})

	store := eventbus.NewStore(nil, nil)
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "my-project",
		WorktreeID:    "main",
		Data:          []byte(`{"notificationType":"permission_prompt"}`),
		Timestamp:     time.Now(),
	})

	svc := New(ServiceDeps{Engine: mock, DB: database, Store: store})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !projects[0].NeedsInput {
		t.Error("expected project needsInput=true from event store")
	}
	if projects[0].NotificationType != "permission_prompt" {
		t.Errorf("expected notificationType 'permission_prompt', got %q", projects[0].NotificationType)
	}
}

func TestListProjects_OverlaysAttentionHighestPriority(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running"},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})

	store := eventbus.NewStore(nil, nil)
	// One worktree needs idle_prompt (low priority)
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "my-project",
		WorktreeID:    "wt-1",
		Data:          []byte(`{"notificationType":"idle_prompt"}`),
		Timestamp:     time.Now(),
	})
	// Another worktree needs permission_prompt (high priority)
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "my-project",
		WorktreeID:    "wt-2",
		Data:          []byte(`{"notificationType":"permission_prompt"}`),
		Timestamp:     time.Now(),
	})

	svc := New(ServiceDeps{Engine: mock, DB: database, Store: store})

	projects, err := svc.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !projects[0].NeedsInput {
		t.Error("expected project needsInput=true")
	}
	if projects[0].NotificationType != "permission_prompt" {
		t.Errorf("expected highest-priority 'permission_prompt', got %q", projects[0].NotificationType)
	}
}

// testDB returns a fresh in-memory SQLite store for tests.
func testDB(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

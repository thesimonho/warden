package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
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
	svc := New(mock, database, nil, nil)

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
	svc := New(mock, database, nil, nil)

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

	svc := New(mock, database, nil, nil)

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

	svc := New(mock, database, nil, nil)

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

	svc := New(mock, database, nil, nil)

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
	svc := New(&mockEngine{}, database, nil, nil)

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
	svc := New(&mockEngine{}, database, nil, nil)

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
	svc := New(&mockEngine{}, database, nil, nil)

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

	svc := New(&mockEngine{}, database, nil, nil)

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
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project"}
	if _, err := svc.StopProject(context.Background(), row); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestStopProject_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{stopErr: errors.New("not found")}
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project"}
	_, err := svc.StopProject(context.Background(), row)
	if err == nil {
		t.Fatal("expected error")
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

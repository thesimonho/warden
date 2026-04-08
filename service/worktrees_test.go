package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/event"
	"github.com/thesimonho/warden/eventbus"
)

// testProjectRowMinimal creates a minimal ProjectRow for worktree tests.
func testProjectRowMinimal(containerID, containerName string) *db.ProjectRow {
	return &db.ProjectRow{
		ProjectID:     containerName,
		ContainerID:   containerID,
		ContainerName: containerName,
		Name:          containerName,
		HostPath:      "/test/" + containerName,
	}
}

// insertTestProject inserts a ProjectRow into the DB for tests that need
// the service to resolve a project by ID.
func insertTestProject(t *testing.T, store *db.Store, row *db.ProjectRow) {
	t.Helper()
	if err := store.InsertProject(*row); err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}
}

func TestListWorktrees(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		worktrees: []engine.Worktree{
			{ID: "main", Path: "/project", Branch: "main", State: engine.WorktreeStateConnected},
			{ID: "feature-x", Path: "/project/.claude/worktrees/feature-x", Branch: "feature-x", State: engine.WorktreeStateStopped},
		},
	}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	worktrees, err := svc.ListWorktrees(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Verify actual worktree content, not just count.
	if worktrees[0].ID != "main" {
		t.Errorf("expected first worktree ID 'main', got %q", worktrees[0].ID)
	}
	if worktrees[0].Branch != "main" {
		t.Errorf("expected first worktree branch 'main', got %q", worktrees[0].Branch)
	}
	if worktrees[1].ID != "feature-x" {
		t.Errorf("expected second worktree ID 'feature-x', got %q", worktrees[1].ID)
	}
	if worktrees[1].State != engine.WorktreeStateStopped {
		t.Errorf("expected second worktree state 'stopped', got %q", worktrees[1].State)
	}
}

func TestListWorktrees_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{worktreesErr: errors.New("container not found")}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.ListWorktrees(context.Background(), row.ProjectID, "claude-code")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestListWorktrees_OverlaysState(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		worktrees: []engine.Worktree{
			{ID: "main", Path: "/project", Branch: "main", State: engine.WorktreeStateConnected},
		},
	}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)

	// Push a terminal connected event so the store has terminal data.
	now := time.Now()
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventTerminalConnected,
		ContainerName: "my-project",
		WorktreeID:    "main",
		Timestamp:     now,
	})

	// Push a session exit to transition to shell state.
	exitData, _ := json.Marshal(map[string]any{"exitCode": 0})
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventSessionExit,
		ContainerName: "my-project",
		WorktreeID:    "main",
		Data:          exitData,
		Timestamp:     now.Add(time.Second),
	})

	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	worktrees, err := svc.ListWorktrees(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The session exited, so the store should derive "shell" state.
	if worktrees[0].State != engine.WorktreeStateShell {
		t.Errorf("expected state 'shell', got %q", worktrees[0].State)
	}
}

func TestCreateWorktree(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		createWorktreeResp: "feature-y",
	}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	resp, err := svc.CreateWorktree(context.Background(), row.ProjectID, "claude-code", "feature-y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "feature-y" {
		t.Errorf("expected worktree ID 'feature-y', got %q", resp.WorktreeID)
	}

	// Verify the engine received the correct worktree name.
	if mock.lastWorktreeName != "feature-y" {
		t.Errorf("expected engine to receive worktree name 'feature-y', got %q", mock.lastWorktreeName)
	}
}

func TestCreateWorktree_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{createWorktreeErr: errors.New("failed")}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.CreateWorktree(context.Background(), row.ProjectID, "claude-code", "feature-y")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCreateWorktree_BroadcastsListChanged(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		createWorktreeResp: "feature-y",
	}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store})

	// Subscribe before creating to catch the broadcast.
	ch, unsub := broker.Subscribe()
	defer unsub()

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.CreateWorktree(context.Background(), row.ProjectID, "claude-code", "feature-y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Drain the channel to find the worktree_list_changed event.
	found := false
	for i := 0; i < 10; i++ {
		select {
		case evt := <-ch:
			if evt.Event == "worktree_list_changed" {
				found = true
			}
		default:
		}
		if found {
			break
		}
	}
	if !found {
		t.Error("expected worktree_list_changed event to be broadcast")
	}
}

func TestConnectTerminal(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		connectResp: "main",
	}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	resp, err := svc.ConnectTerminal(context.Background(), row.ProjectID, "claude-code", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "main" {
		t.Errorf("expected worktree ID 'main', got %q", resp.WorktreeID)
	}

	// Verify the store has terminal data after connect.
	if !store.HasTerminalData("my-project") {
		t.Error("expected store to have terminal data after connect")
	}
}

func TestRemoveWorktree(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.RemoveWorktree(context.Background(), row.ProjectID, "claude-code", "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveWorktree_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{removeWorktreeErr: errors.New("cannot remove main")}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.RemoveWorktree(context.Background(), row.ProjectID, "claude-code", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestCleanupWorktrees(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		cleanupRemoved: []string{"stale-1", "stale-2"},
	}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	removed, err := svc.CleanupWorktrees(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(removed) != 2 {
		t.Fatalf("expected 2 removed, got %d", len(removed))
	}
}

func TestKillWorktreeProcess(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.KillWorktreeProcess(context.Background(), row.ProjectID, "claude-code", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKillWorktreeProcess_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{killWorktreeErr: errors.New("process not found")}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.KillWorktreeProcess(context.Background(), row.ProjectID, "claude-code", "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDisconnectTerminal(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := testProjectRowMinimal("abc123def456", "my-project")
	insertTestProject(t, database, row)
	_, err := svc.DisconnectTerminal(context.Background(), row.ProjectID, "claude-code", "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

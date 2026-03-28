package service

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
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

func TestListWorktrees(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		worktrees: []engine.Worktree{
			{ID: "main", Path: "/project", Branch: "main", State: engine.WorktreeStateConnected},
			{ID: "feature-x", Path: "/project/.claude/worktrees/feature-x", Branch: "feature-x", State: engine.WorktreeStateDisconnected},
		},
	}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	worktrees, err := svc.ListWorktrees(context.Background(), row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}
}

func TestListWorktrees_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{worktreesErr: errors.New("container not found")}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.ListWorktrees(context.Background(), row)
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
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventTerminalConnected,
		ContainerName: "my-project",
		WorktreeID:    "main",
		Timestamp:     now,
	})

	// Push a session exit to transition to shell state.
	exitData, _ := json.Marshal(map[string]any{"exitCode": 0})
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventSessionExit,
		ContainerName: "my-project",
		WorktreeID:    "main",
		Data:          exitData,
		Timestamp:     now.Add(time.Second),
	})

	svc := New(mock, testDB(t), store, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	worktrees, err := svc.ListWorktrees(context.Background(), row)
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
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	resp, err := svc.CreateWorktree(context.Background(), row, "feature-y")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "feature-y" {
		t.Errorf("expected worktree ID 'feature-y', got %q", resp.WorktreeID)
	}
}

func TestCreateWorktree_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{createWorktreeErr: errors.New("failed")}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.CreateWorktree(context.Background(), row, "feature-y")
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
	svc := New(mock, testDB(t), store, nil)

	// Subscribe before creating to catch the broadcast.
	ch, unsub := broker.Subscribe()
	defer unsub()

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.CreateWorktree(context.Background(), row, "feature-y")
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
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	resp, err := svc.ConnectTerminal(context.Background(), row, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "main" {
		t.Errorf("expected worktree ID 'main', got %q", resp.WorktreeID)
	}
}

func TestConnectTerminal_PushesEvent(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{
		connectResp: "main",
	}

	broker := eventbus.NewBroker()
	defer broker.Shutdown()
	store := eventbus.NewStore(broker, nil)
	svc := New(mock, testDB(t), store, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.ConnectTerminal(context.Background(), row, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
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
	svc := New(mock, testDB(t), store, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.RemoveWorktree(context.Background(), row, "feature-x")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRemoveWorktree_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{removeWorktreeErr: errors.New("cannot remove main")}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.RemoveWorktree(context.Background(), row, "main")
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
	svc := New(mock, testDB(t), store, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	removed, err := svc.CleanupWorktrees(context.Background(), row)
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
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.KillWorktreeProcess(context.Background(), row, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestKillWorktreeProcess_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{killWorktreeErr: errors.New("process not found")}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.KillWorktreeProcess(context.Background(), row, "main")
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestDisconnectTerminal(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}
	svc := New(mock, testDB(t), nil, nil)

	row := testProjectRowMinimal("abc123def456", "my-project")
	_, err := svc.DisconnectTerminal(context.Background(), row, "main")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

// enforceBudgetTestSetup creates a service with a project that has a $10
// budget and two worktrees, wired to a mock engine that can track calls.
func enforceBudgetTestSetup(t *testing.T) (*Service, *mockEngine) {
	t.Helper()
	store := testDB(t)

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "ctr123", Name: "test-project"},
		},
		worktrees: []engine.Worktree{
			{ID: "wt-1"},
			{ID: "wt-2"},
		},
	}
	svc := New(mock, store, nil, nil)

	// Set a $10 per-project budget.
	_ = store.InsertProject(testProjectRow("test-project", 10.00))

	return svc, mock
}

// setBudgetAction sets a single budget action setting in the DB.
func setBudgetAction(svc *Service, key, value string) {
	_ = svc.db.SetSetting(key, value)
}

func TestPersistSessionCost_WarnOnly(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	// Default: warn=true, others=false.
	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	if mock.stopCalled {
		t.Error("container should not be stopped when stopContainer is off")
	}
	if len(mock.killedWorktrees) > 0 {
		t.Error("worktrees should not be killed when stopWorktrees is off")
	}
}

func TestPersistSessionCost_StopWorktrees(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopWorktrees, "true")

	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	if len(mock.killedWorktrees) != 2 {
		t.Errorf("expected 2 worktrees killed, got %d", len(mock.killedWorktrees))
	}
	if mock.stopCalled {
		t.Error("container should not be stopped when stopContainer is off")
	}
}

func TestPersistSessionCost_StopContainer(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	if !mock.stopCalled {
		t.Error("container should be stopped when stopContainer is on")
	}
	// Worktrees should NOT be killed (stopWorktrees is off).
	if len(mock.killedWorktrees) > 0 {
		t.Error("worktrees should not be killed when stopWorktrees is off")
	}
}

func TestPersistSessionCost_AllActions(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionWarn, "true")
	setBudgetAction(svc, settingBudgetActionStopWorktrees, "true")
	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	if len(mock.killedWorktrees) != 2 {
		t.Errorf("expected 2 worktrees killed, got %d", len(mock.killedWorktrees))
	}
	if !mock.stopCalled {
		t.Error("container should be stopped")
	}
}

func TestPersistSessionCost_WithinBudget(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopWorktrees, "true")
	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	// Cost within budget — no actions should fire.
	svc.PersistSessionCost("test-project", "test-project", "session-1", 5.00, false)

	if len(mock.killedWorktrees) > 0 {
		t.Error("no worktrees should be killed when within budget")
	}
	if mock.stopCalled {
		t.Error("container should not be stopped when within budget")
	}
}

func TestPersistSessionCost_NoBudget(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	mock := &mockEngine{}
	svc := New(mock, store, nil, nil)

	// No budget set — should be a no-op even with all actions enabled.
	setBudgetAction(svc, settingBudgetActionStopWorktrees, "true")
	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	svc.PersistSessionCost("unknown-project", "unknown-project", "session-1", 50.00, false)

	if mock.stopCalled {
		t.Error("should not stop when no budget is set")
	}
}

func TestPersistSessionCost_EmptySessionIDStillEnforces(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	// Pre-seed cost in DB directly (simulates previously persisted data).
	_ = svc.db.UpsertSessionCost("test-project", "session-1", 15.00, false)

	// Call with empty session ID — no DB write, but enforcement still runs
	// against the previously persisted data.
	svc.PersistSessionCost("test-project", "test-project", "", 0, false)

	if !mock.stopCalled {
		t.Error("enforcement must run even when session ID is empty")
	}
}

func TestPersistSessionCost_ZeroCostStillEnforces(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	// Pre-seed cost in DB.
	_ = svc.db.UpsertSessionCost("test-project", "session-1", 15.00, false)

	// Call with zero cost — no DB write, but enforcement still runs.
	svc.PersistSessionCost("test-project", "test-project", "session-2", 0, false)

	if !mock.stopCalled {
		t.Error("enforcement must run even when cost is zero")
	}
}

func TestIsOverBudget_PreventStartEnabled(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	svc := New(&mockEngine{}, store, nil, nil)

	_ = store.InsertProject(testProjectRow("proj", 10.00))
	_ = store.UpsertSessionCost("proj", "session-1", 15.00, false)

	setBudgetAction(svc, settingBudgetActionPreventStart, "true")

	if !svc.IsOverBudget("proj") {
		t.Error("expected IsOverBudget to return true")
	}
}

func TestIsOverBudget_PreventStartDisabled(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	svc := New(&mockEngine{}, store, nil, nil)

	_ = store.InsertProject(testProjectRow("proj", 10.00))
	_ = store.UpsertSessionCost("proj", "session-1", 15.00, false)

	// preventStart defaults to false — should not block.
	if svc.IsOverBudget("proj") {
		t.Error("expected IsOverBudget to return false when preventStart is off")
	}
}

func TestIsOverBudget_WithinBudget(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	svc := New(&mockEngine{}, store, nil, nil)

	_ = store.InsertProject(testProjectRow("proj", 10.00))
	_ = store.UpsertSessionCost("proj", "session-1", 5.00, false)

	setBudgetAction(svc, settingBudgetActionPreventStart, "true")

	if svc.IsOverBudget("proj") {
		t.Error("expected IsOverBudget to return false when within budget")
	}
}

func TestRestartProject_BlockedByBudget(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "ctr123", Name: "proj"},
		},
		inspectConfig: &engine.ContainerConfig{Name: "proj"},
	}
	svc := New(mock, store, nil, nil)

	_ = store.InsertProject(testProjectRow("proj", 10.00))
	_ = store.UpsertSessionCost("proj", "session-1", 15.00, false)
	setBudgetAction(svc, settingBudgetActionPreventStart, "true")

	row := &db.ProjectRow{ProjectID: "proj", ContainerID: "ctr123", ContainerName: "proj", Name: "proj"}
	_, err := svc.RestartProject(context.Background(), row)
	if !errors.Is(err, ErrBudgetExceeded) {
		t.Errorf("expected ErrBudgetExceeded, got %v", err)
	}
}

func TestPersistSessionCost_MultiWorktreeAggregateExceedsBudget(t *testing.T) {
	t.Parallel()
	svc, mock := enforceBudgetTestSetup(t)

	setBudgetAction(svc, settingBudgetActionStopContainer, "true")

	// Persist two session costs that individually are under the $10 budget
	// but together exceed it: $6 + $6 = $12. The second PersistSessionCost
	// should trigger enforcement because the aggregate exceeds the budget.
	svc.PersistSessionCost("test-project", "test-project", "session-1", 6.00, false)

	if mock.stopCalled {
		t.Error("container should not be stopped when single session is within budget")
	}

	svc.PersistSessionCost("test-project", "test-project", "session-2", 6.00, false)

	if !mock.stopCalled {
		t.Error("container should be stopped when DB aggregate exceeds budget")
	}
}

func TestPersistSessionCost_WritesAuditEntries(t *testing.T) {
	t.Parallel()
	store := testDB(t)

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "ctr123", Name: "test-project"},
		},
		worktrees: []engine.Worktree{
			{ID: "wt-1"},
		},
	}

	audit := db.NewAuditWriter(store, db.AuditDetailed, nil)
	svc := New(mock, store, nil, audit)

	_ = store.InsertProject(testProjectRow("test-project", 10.00))

	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	entries, err := store.Query(db.QueryFilters{Event: "budget_exceeded"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 budget_exceeded entry, got %d", len(entries))
	}
	if entries[0].ProjectID != "test-project" {
		t.Errorf("expected projectID test-project, got %q", entries[0].ProjectID)
	}
}

func TestPersistSessionCost_WorksWhenAuditOff(t *testing.T) {
	t.Parallel()
	store := testDB(t)

	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "ctr123", Name: "test-project"},
		},
		worktrees: []engine.Worktree{
			{ID: "wt-1"},
			{ID: "wt-2"},
		},
	}

	audit := db.NewAuditWriter(store, db.AuditOff, nil)
	svc := New(mock, store, nil, audit)

	_ = store.InsertProject(testProjectRow("test-project", 10.00))
	setBudgetAction(svc, settingBudgetActionStopWorktrees, "true")

	svc.PersistSessionCost("test-project", "test-project", "session-1", 15.00, false)

	// Enforcement action should still fire even with audit off.
	if len(mock.killedWorktrees) != 2 {
		t.Errorf("expected 2 worktrees killed, got %d", len(mock.killedWorktrees))
	}

	// No audit entries should be written.
	entries, err := store.Query(db.QueryFilters{Event: "budget_exceeded"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 audit entries in off mode, got %d", len(entries))
	}
}

func TestRestartProject_AllowedWhenPreventStartOff(t *testing.T) {
	t.Parallel()
	store := testDB(t)
	mock := &mockEngine{
		projects: []engine.Project{
			{ID: "ctr123", Name: "proj"},
		},
		inspectConfig: &engine.ContainerConfig{Name: "proj"},
	}
	svc := New(mock, store, nil, nil)

	_ = store.InsertProject(testProjectRow("proj", 10.00))
	_ = store.UpsertSessionCost("proj", "session-1", 15.00, false)
	// preventStart is off by default — restart should proceed.

	row := &db.ProjectRow{ProjectID: "proj", ContainerID: "ctr123", ContainerName: "proj", Name: "proj"}
	_, err := svc.RestartProject(context.Background(), row)
	// Restart itself may fail (no real container), but it should NOT
	// be ErrBudgetExceeded.
	if errors.Is(err, ErrBudgetExceeded) {
		t.Error("restart should not be blocked when preventStart is off")
	}
}

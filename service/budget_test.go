package service

import (
	"testing"
	"time"

	"github.com/thesimonho/warden/db"
)

// testProjectRow creates a minimal ProjectRow for testing with a given budget.
// ProjectID is set equal to name for simplicity in tests (production uses
// a sha256 hash). ContainerID is set to "abc123def456" for budget enforcement
// tests that need to resolve the container.
func testProjectRow(name string, costBudget float64) db.ProjectRow {
	return db.ProjectRow{
		ProjectID:     name,
		Name:          name,
		HostPath:      "/test/" + name,
		AddedAt:       time.Now().UTC(),
		CostBudget:    costBudget,
		ContainerID:   "abc123def456",
		ContainerName: name,
	}
}

func TestGetEffectiveBudget_PerProjectBudget(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	svc := New(&mockEngine{}, db, nil, nil)

	// Insert a project with a per-project budget.
	_ = db.InsertProject(testProjectRow("budgeted", 25.00))

	budget := svc.GetEffectiveBudget("budgeted")
	if budget != 25.00 {
		t.Errorf("expected 25.00, got %f", budget)
	}
}

func TestGetEffectiveBudget_FallsBackToGlobalDefault(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	svc := New(&mockEngine{}, db, nil, nil)

	// Set global default budget.
	_ = db.SetSetting("defaultProjectBudget", "10")

	// Insert a project with no per-project budget.
	_ = db.InsertProject(testProjectRow("no-budget", 0))

	budget := svc.GetEffectiveBudget("no-budget")
	if budget != 10.00 {
		t.Errorf("expected global default 10.00, got %f", budget)
	}
}

func TestGetEffectiveBudget_Unlimited(t *testing.T) {
	t.Parallel()

	db := testDB(t)
	svc := New(&mockEngine{}, db, nil, nil)

	// No project in DB, no global default.
	budget := svc.GetEffectiveBudget("unknown")
	if budget != 0 {
		t.Errorf("expected 0 (unlimited), got %f", budget)
	}
}

func TestGetEffectiveBudget_NilDB(t *testing.T) {
	t.Parallel()

	svc := New(&mockEngine{}, nil, nil, nil)

	budget := svc.GetEffectiveBudget("anything")
	if budget != 0 {
		t.Errorf("expected 0 (unlimited) with nil DB, got %f", budget)
	}
}

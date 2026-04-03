package warden

import (
	"context"
	"testing"
)

// newTestWarden creates a Warden with a temporary DB directory for testing,
// ensuring tests don't read or mutate the user's real database.
func newTestWarden(t *testing.T, opts Options) *Warden {
	t.Helper()
	if opts.DBDir == "" {
		opts.DBDir = t.TempDir()
	}
	w, err := New(opts)
	if err != nil {
		t.Fatalf("failed to create test warden: %v", err)
	}
	t.Cleanup(w.Close)
	return w
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	w := newTestWarden(t, Options{})

	if w.Service == nil {
		t.Error("expected non-nil Service")
	}
	if w.Broker == nil {
		t.Error("expected non-nil Broker")
	}
	if w.DB == nil {
		t.Error("expected non-nil DB")
	}
	if w.Engine == nil {
		t.Error("expected non-nil Engine")
	}
}

func TestNew_ServiceWorks(t *testing.T) {
	t.Parallel()

	w := newTestWarden(t, Options{})

	// ListProjects should return empty list since no projects configured.
	projects, err := w.Service.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestWarden_CloseIdempotent(t *testing.T) {
	t.Parallel()

	w := newTestWarden(t, Options{})

	// Should not panic on double close.
	w.Close()
	w.Close()
}


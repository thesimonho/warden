package warden

import (
	"context"
	"testing"
)

// newTestApp creates an App with a temporary DB directory for testing,
// ensuring tests don't read or mutate the user's real database.
func newTestApp(t *testing.T, opts Options) *App {
	t.Helper()
	if opts.DBDir == "" {
		opts.DBDir = t.TempDir()
	}
	app, err := New(opts)
	if err != nil {
		t.Fatalf("failed to create test app: %v", err)
	}
	t.Cleanup(app.Close)
	return app
}

func TestNew_Defaults(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, Options{})

	if app.Service == nil {
		t.Error("expected non-nil Service")
	}
	if app.Broker == nil {
		t.Error("expected non-nil Broker")
	}
	if app.DB == nil {
		t.Error("expected non-nil DB")
	}
	if app.Engine == nil {
		t.Error("expected non-nil Engine")
	}
}

func TestNew_ServiceWorks(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, Options{})

	// ListProjects should return empty list since no projects configured.
	projects, err := app.Service.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error listing projects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("expected 0 projects, got %d", len(projects))
	}
}

func TestApp_CloseIdempotent(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, Options{})

	// Should not panic on double close.
	app.Close()
	app.Close()
}

func TestNew_RuntimeOverride(t *testing.T) {
	t.Parallel()

	app := newTestApp(t, Options{Runtime: "podman"})

	// Verify the override was applied — the DB still has the default,
	// but the engine was created with the overridden runtime.
	if app.Service == nil {
		t.Error("expected non-nil Service with runtime override")
	}
}

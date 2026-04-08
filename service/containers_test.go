package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

func TestCreateContainer(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	mock := &mockEngine{containerID: "new123container"}
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	result, err := svc.CreateContainer(context.Background(), api.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ContainerID != "new123container" {
		t.Errorf("expected container ID 'new123container', got %q", result.ContainerID)
	}
	if result.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", result.Name)
	}

	// Verify auto-add to database with computed project ID.
	projectID, _ := engine.ProjectID("/home/user/project")
	has, _ := database.HasProject(projectID, "claude-code")
	if !has {
		t.Error("expected project to be auto-added to database")
	}
	if result.ProjectID != projectID {
		t.Errorf("expected projectID %q, got %q", projectID, result.ProjectID)
	}
}

func TestCreateContainer_NameTaken(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{containerErr: engine.ErrNameTaken}
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)})

	_, err := svc.CreateContainer(context.Background(), api.CreateContainerRequest{
		Name:        "existing",
		ProjectPath: "/home/user/project",
	})
	if !errors.Is(err, engine.ErrNameTaken) {
		t.Fatalf("expected ErrNameTaken, got %v", err)
	}
}

func TestDeleteContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	_, err := svc.DeleteContainer(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInspectContainer(t *testing.T) {
	t.Parallel()

	row := &db.ProjectRow{
		ProjectID:       "my-project",
		ContainerID:     "abc123def456",
		ContainerName:   "my-project",
		Name:            "my-project",
		HostPath:        "/test/my-project",
		SkipPermissions: true,
		NetworkMode:     "restricted",
		AllowedDomains:  "example.com,test.com",
	}

	mock := &mockEngine{
		inspectConfig: &api.ContainerConfig{Name: "my-project"},
	}
	database := testDB(t)
	insertTestProject(t, database, row)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	cfg, err := svc.InspectContainer(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", cfg.Name)
	}
	if !cfg.SkipPermissions {
		t.Error("expected skipPermissions=true from project row overlay")
	}
	if cfg.NetworkMode != api.NetworkModeRestricted {
		t.Errorf("expected networkMode 'restricted', got %q", cfg.NetworkMode)
	}
	if len(cfg.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(cfg.AllowedDomains))
	}
}

func TestUpdateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{recreateID: "new456container"}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "old123container", ContainerName: "my-project", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	result, err := svc.UpdateContainer(context.Background(), row.ProjectID, "claude-code", api.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.ContainerID != "new456container" {
		t.Errorf("expected container ID 'new456container', got %q", result.ContainerID)
	}
}

func TestUpdateContainer_LightUpdate(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	mock := &mockEngine{containerID: "container123"}
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	// Create the project first so the DB row exists.
	_, err := svc.CreateContainer(context.Background(), api.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
		Image:       "ghcr.io/test:latest",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	projectID, _ := engine.ProjectID("/home/user/project")
	row, _ := database.GetProject(projectID, "claude-code")

	// Update only lightweight fields (name, skipPermissions, costBudget).
	result, err := svc.UpdateContainer(context.Background(), row.ProjectID, "claude-code", api.CreateContainerRequest{
		Name:            "renamed-project",
		ProjectPath:     "/home/user/project",
		Image:           "ghcr.io/test:latest",
		SkipPermissions: true,
		CostBudget:      25.0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should reuse existing container ID (no recreation).
	if result.ContainerID != "container123" {
		t.Errorf("expected existing container ID 'container123', got %q", result.ContainerID)
	}
	if result.Name != "renamed-project" {
		t.Errorf("expected name 'renamed-project', got %q", result.Name)
	}

	// Verify DB was updated.
	updated, _ := database.GetProject(projectID, "claude-code")
	if updated.Name != "renamed-project" {
		t.Errorf("expected DB name 'renamed-project', got %q", updated.Name)
	}
	if !updated.SkipPermissions {
		t.Error("expected DB skipPermissions=true")
	}
	if updated.CostBudget != 25.0 {
		t.Errorf("expected DB costBudget=25.0, got %f", updated.CostBudget)
	}
}

func TestUpdateContainer_LightUpdate_RenameError(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	mock := &mockEngine{
		containerID: "container123",
		renameErr:   errors.New("rename failed"),
	}
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	_, err := svc.CreateContainer(context.Background(), api.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
		Image:       "ghcr.io/test:latest",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	projectID, _ := engine.ProjectID("/home/user/project")
	row, _ := database.GetProject(projectID, "claude-code")

	// Rename should fail and propagate.
	_, err = svc.UpdateContainer(context.Background(), row.ProjectID, "claude-code", api.CreateContainerRequest{
		Name:        "new-name",
		ProjectPath: "/home/user/project",
		Image:       "ghcr.io/test:latest",
	})
	if err == nil {
		t.Fatal("expected error from rename failure")
	}
}

func TestNeedsRecreation(t *testing.T) {
	t.Parallel()

	base := &db.ProjectRow{
		HostPath:    "/home/user/project",
		Image:       "ghcr.io/test:latest",
		AgentType:   "claude-code",
		NetworkMode: "full",
	}

	tests := []struct {
		name   string
		req    api.CreateContainerRequest
		expect bool
	}{
		{
			name: "only name changed",
			req: api.CreateContainerRequest{
				Name:        "new-name",
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
			},
			expect: false,
		},
		{
			name: "only skipPermissions changed",
			req: api.CreateContainerRequest{
				ProjectPath:     "/home/user/project",
				Image:           "ghcr.io/test:latest",
				SkipPermissions: true,
			},
			expect: false,
		},
		{
			name: "only costBudget changed",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				CostBudget:  50.0,
			},
			expect: false,
		},
		{
			name: "image changed",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:v2",
			},
			expect: true,
		},
		{
			name: "project path changed",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/other",
				Image:       "ghcr.io/test:latest",
			},
			expect: true,
		},
		{
			name: "agent type changed",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				AgentType:   "codex",
			},
			expect: true,
		},
		{
			name: "network mode changed",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				NetworkMode: api.NetworkModeRestricted,
			},
			expect: true,
		},
		{
			name: "env vars added",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				EnvVars:     map[string]string{"FOO": "bar"},
			},
			expect: true,
		},
		{
			name: "mounts added",
			req: api.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				Mounts:      []api.Mount{{HostPath: "/a", ContainerPath: "/b"}},
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := needsRecreation(base, tt.req)
			if got != tt.expect {
				t.Errorf("needsRecreation() = %v, want %v", got, tt.expect)
			}
		})
	}
}

func TestMergeRuntimeDomains_IncludesSystemDomains(t *testing.T) {
	t.Parallel()

	req := api.CreateContainerRequest{
		NetworkMode: api.NetworkModeRestricted,
		AllowedDomains: []string{"example.com"},
	}
	mergeRuntimeDomains(&req)

	has := make(map[string]bool)
	for _, d := range req.AllowedDomains {
		has[d] = true
	}
	if !has["storage.googleapis.com"] {
		t.Error("expected storage.googleapis.com (system domain) in merged domains")
	}
	if !has["example.com"] {
		t.Error("expected user domain example.com to be preserved")
	}
}

func TestMergeRuntimeDomains_SkipsForFullMode(t *testing.T) {
	t.Parallel()

	req := api.CreateContainerRequest{
		NetworkMode:    api.NetworkModeFull,
		AllowedDomains: []string{"example.com"},
	}
	mergeRuntimeDomains(&req)

	if len(req.AllowedDomains) != 1 {
		t.Errorf("expected 1 domain for full mode, got %d: %v", len(req.AllowedDomains), req.AllowedDomains)
	}
}

func TestMergeRuntimeDomains_IncludesRuntimeAndSystemDomains(t *testing.T) {
	t.Parallel()

	req := api.CreateContainerRequest{
		NetworkMode:     api.NetworkModeRestricted,
		AllowedDomains:  []string{"example.com"},
		EnabledRuntimes: []string{"python"},
	}
	mergeRuntimeDomains(&req)

	has := make(map[string]bool)
	for _, d := range req.AllowedDomains {
		has[d] = true
	}
	if !has["storage.googleapis.com"] {
		t.Error("expected system domain storage.googleapis.com")
	}
	if !has["pypi.org"] {
		t.Error("expected python runtime domain pypi.org")
	}
	if !has["example.com"] {
		t.Error("expected user domain example.com to be preserved")
	}
}

func TestHandleContainerStart_SkipsRecentlyCreated(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: &mockEngine{}, DB: database})

	// Insert a project with restricted network mode.
	row := &db.ProjectRow{
		ProjectID:     "proj-1",
		ContainerID:   "abc123def456",
		ContainerName: "my-project",
		Name:          "my-project",
		HostPath:      "/test/my-project",
		NetworkMode:   "restricted",
		AllowedDomains: "example.com",
	}
	insertTestProject(t, database, row)

	// Mark as recently created with 12-char ID (as CreateContainer does).
	svc.recentlyCreated.Store("abc123def456", true)

	// Docker events provide full 64-char IDs — HandleContainerStart
	// must truncate before lookup.
	fullID := "abc123def456789abcdef0123456789abcdef0123456789abcdef0123456789ab"
	svc.HandleContainerStart(fullID, "my-project")

	// The entry should be consumed (deleted).
	if _, loaded := svc.recentlyCreated.Load("abc123def456"); loaded {
		t.Error("expected recentlyCreated entry to be consumed after HandleContainerStart")
	}
}

func TestHandleContainerStart_AppliesOnRestart(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	mock := &mockEngine{}
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	// Insert a project with restricted network mode.
	row := &db.ProjectRow{
		ProjectID:     "proj-1",
		ContainerID:   "abc123def456",
		ContainerName: "my-project",
		Name:          "my-project",
		HostPath:      "/test/my-project",
		NetworkMode:   "restricted",
		AllowedDomains: "example.com",
	}
	insertTestProject(t, database, row)

	// Do NOT mark as recently created — simulates a Docker auto-restart.
	// HandleContainerStart spawns a goroutine for the isolation work.
	svc.HandleContainerStart("abc123def456", "my-project")

	// Wait for the goroutine to complete (mock WaitForInstalls returns immediately).
	time.Sleep(100 * time.Millisecond)

	// Should have called ApplyNetworkIsolation on the mock.
	if !mock.networkIsolationApplied {
		t.Error("expected ApplyNetworkIsolation to be called on restart")
	}
}

func TestValidateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{validateValid: true, validateMissing: []string{}}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	result, err := svc.ValidateContainer(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true")
	}
}

func TestValidateContainer_Missing(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{validateValid: false, validateMissing: []string{"tmux", "create-terminal.sh"}}
	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: mock, DB: database})

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", Name: "my-project", HostPath: "/test/my-project"}
	insertTestProject(t, database, row)
	result, err := svc.ValidateContainer(context.Background(), row.ProjectID, "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false")
	}
	if len(result.Missing) != 2 {
		t.Fatalf("expected 2 missing, got %d", len(result.Missing))
	}
	if result.Missing[0] != "tmux" {
		t.Errorf("expected first missing binary 'tmux', got %q", result.Missing[0])
	}
	if result.Missing[1] != "create-terminal.sh" {
		t.Errorf("expected second missing binary 'create-terminal.sh', got %q", result.Missing[1])
	}
}

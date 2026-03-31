package service

import (
	"context"
	"errors"
	"testing"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

func TestCreateContainer(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	mock := &mockEngine{containerID: "new123container"}
	svc := New(mock, database, nil, nil)

	result, err := svc.CreateContainer(context.Background(), engine.CreateContainerRequest{
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
	has, _ := database.HasProject(projectID)
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
	svc := New(mock, testDB(t), nil, nil)

	_, err := svc.CreateContainer(context.Background(), engine.CreateContainerRequest{
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
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", ContainerName: "my-project", Name: "my-project"}
	_, err := svc.DeleteContainer(context.Background(), row)
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
		inspectConfig: &engine.ContainerConfig{Name: "my-project"},
	}
	svc := New(mock, testDB(t), nil, nil)

	cfg, err := svc.InspectContainer(context.Background(), row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", cfg.Name)
	}
	if !cfg.SkipPermissions {
		t.Error("expected skipPermissions=true from project row overlay")
	}
	if cfg.NetworkMode != engine.NetworkModeRestricted {
		t.Errorf("expected networkMode 'restricted', got %q", cfg.NetworkMode)
	}
	if len(cfg.AllowedDomains) != 2 {
		t.Errorf("expected 2 allowed domains, got %d", len(cfg.AllowedDomains))
	}
}

func TestUpdateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{recreateID: "new456container"}
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "old123container", ContainerName: "my-project", Name: "my-project"}
	result, err := svc.UpdateContainer(context.Background(), row, engine.CreateContainerRequest{
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
	svc := New(mock, database, nil, nil)

	// Create the project first so the DB row exists.
	_, err := svc.CreateContainer(context.Background(), engine.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
		Image:       "ghcr.io/test:latest",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	projectID, _ := engine.ProjectID("/home/user/project")
	row, _ := database.GetProject(projectID)

	// Update only lightweight fields (name, skipPermissions, costBudget).
	result, err := svc.UpdateContainer(context.Background(), row, engine.CreateContainerRequest{
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
	updated, _ := database.GetProject(projectID)
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
	svc := New(mock, database, nil, nil)

	_, err := svc.CreateContainer(context.Background(), engine.CreateContainerRequest{
		Name:        "my-project",
		ProjectPath: "/home/user/project",
		Image:       "ghcr.io/test:latest",
	})
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	projectID, _ := engine.ProjectID("/home/user/project")
	row, _ := database.GetProject(projectID)

	// Rename should fail and propagate.
	_, err = svc.UpdateContainer(context.Background(), row, engine.CreateContainerRequest{
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
		req    engine.CreateContainerRequest
		expect bool
	}{
		{
			name: "only name changed",
			req: engine.CreateContainerRequest{
				Name:        "new-name",
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
			},
			expect: false,
		},
		{
			name: "only skipPermissions changed",
			req: engine.CreateContainerRequest{
				ProjectPath:     "/home/user/project",
				Image:           "ghcr.io/test:latest",
				SkipPermissions: true,
			},
			expect: false,
		},
		{
			name: "only costBudget changed",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				CostBudget:  50.0,
			},
			expect: false,
		},
		{
			name: "image changed",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:v2",
			},
			expect: true,
		},
		{
			name: "project path changed",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/other",
				Image:       "ghcr.io/test:latest",
			},
			expect: true,
		},
		{
			name: "agent type changed",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				AgentType:   "codex",
			},
			expect: true,
		},
		{
			name: "network mode changed",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				NetworkMode: engine.NetworkModeRestricted,
			},
			expect: true,
		},
		{
			name: "env vars added",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				EnvVars:     map[string]string{"FOO": "bar"},
			},
			expect: true,
		},
		{
			name: "mounts added",
			req: engine.CreateContainerRequest{
				ProjectPath: "/home/user/project",
				Image:       "ghcr.io/test:latest",
				Mounts:      []engine.Mount{{HostPath: "/a", ContainerPath: "/b"}},
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

func TestValidateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{validateValid: true, validateMissing: []string{}}
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", Name: "my-project"}
	result, err := svc.ValidateContainer(context.Background(), row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !result.Valid {
		t.Error("expected valid=true")
	}
}

func TestValidateContainer_Missing(t *testing.T) {
	t.Parallel()

	mock := &mockEngine{validateValid: false, validateMissing: []string{"abduco", "create-terminal.sh"}}
	svc := New(mock, testDB(t), nil, nil)

	row := &db.ProjectRow{ProjectID: "proj-1", ContainerID: "abc123def456", Name: "my-project"}
	result, err := svc.ValidateContainer(context.Background(), row)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false")
	}
	if len(result.Missing) != 2 {
		t.Errorf("expected 2 missing, got %d", len(result.Missing))
	}
}

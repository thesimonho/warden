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

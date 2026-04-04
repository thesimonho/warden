package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/service"
)

func TestCreateContainer(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/container",
		http.StatusCreated, service.ContainerResult{ContainerID: "ctr123", Name: "my-project"})

	resp, err := c.CreateContainer(context.Background(), "abc123def456", "claude-code", api.CreateContainerRequest{
		ProjectPath: "/home/user/project",
		Name:        "my-project",
		NetworkMode: api.NetworkModeFull,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContainerID != "ctr123" {
		t.Errorf("expected container ID 'ctr123', got %q", resp.ContainerID)
	}
}

func TestInspectContainer(t *testing.T) {
	t.Parallel()

	cfg := api.ContainerConfig{
		Name:        "my-project",
		Image:       "ghcr.io/thesimonho/warden:latest",
		NetworkMode: api.NetworkModeFull,
	}
	c := newTestServer(t, "GET", "/api/v1/projects/abc123def456/claude-code/container/config",
		http.StatusOK, cfg)

	resp, err := c.InspectContainer(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", resp.Name)
	}
	if resp.NetworkMode != api.NetworkModeFull {
		t.Errorf("expected network mode 'full', got %q", resp.NetworkMode)
	}
}

func TestUpdateContainer(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "PUT", "/api/v1/projects/abc123def456/claude-code/container",
		http.StatusOK, service.ContainerResult{ContainerID: "ctr456", Name: "my-project"})

	resp, err := c.UpdateContainer(context.Background(), "abc123def456", "claude-code", api.CreateContainerRequest{
		ProjectPath: "/home/user/project",
		NetworkMode: api.NetworkModeRestricted,
		AllowedDomains: []string{"github.com"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ContainerID != "ctr456" {
		t.Errorf("expected container ID 'ctr456', got %q", resp.ContainerID)
	}
}

func TestValidateContainer(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "GET", "/api/v1/projects/abc123def456/claude-code/container/validate",
		http.StatusOK, api.ValidateContainerResult{Valid: true})

	resp, err := c.ValidateContainer(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.Valid {
		t.Error("expected container to be valid")
	}
}

func TestResetProjectCosts(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/projects/abc123def456/claude-code/costs",
		http.StatusNoContent, nil)

	err := c.ResetProjectCosts(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPurgeProjectAudit(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/projects/abc123def456/claude-code/audit",
		http.StatusNoContent, nil)

	err := c.PurgeProjectAudit(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

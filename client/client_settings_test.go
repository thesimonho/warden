package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestGetSettings(t *testing.T) {
	t.Parallel()

	settings := api.SettingsResponse{
		AuditLogMode: "standard",
	}
	c := newTestServer(t, "GET", "/api/v1/settings", http.StatusOK, settings)

	resp, err := c.GetSettings(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.AuditLogMode != "standard" {
		t.Errorf("expected audit mode 'standard', got %q", resp.AuditLogMode)
	}
}

func TestUpdateSettings(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "PUT", "/api/v1/settings",
		http.StatusOK, api.UpdateSettingsResult{RestartRequired: true})

	mode := api.AuditLogDetailed
	resp, err := c.UpdateSettings(context.Background(), api.UpdateSettingsRequest{
		AuditLogMode: &mode,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resp.RestartRequired {
		t.Error("expected restart required")
	}
}

func TestReadProjectTemplate(t *testing.T) {
	t.Parallel()

	tmpl := api.ProjectTemplate{
		Image: "custom:latest",
	}
	c := newTestServer(t, "GET", "/api/v1/template", http.StatusOK, tmpl)

	resp, err := c.ReadProjectTemplate(context.Background(), "/path/to/.warden.json")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Image != "custom:latest" {
		t.Errorf("expected image 'custom:latest', got %q", resp.Image)
	}
}

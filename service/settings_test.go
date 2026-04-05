package service

import (
	"context"
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestGetSettings(t *testing.T) {
	t.Parallel()

	svc := New(ServiceDeps{DockerAvailable: true, Engine: &mockEngine{}, DB: testDB(t)})

	settings := svc.GetSettings()
	if settings.AuditLogMode != api.AuditLogOff {
		t.Errorf("expected auditLogMode=%q, got %q", api.AuditLogOff, settings.AuditLogMode)
	}
	if settings.Runtime != "docker" {
		t.Errorf("expected runtime 'docker', got %q", settings.Runtime)
	}
}

func TestUpdateSettings_AuditLogMode(t *testing.T) {
	t.Parallel()

	database := testDB(t)
	svc := New(ServiceDeps{DockerAvailable: true, Engine: &mockEngine{}, DB: database})

	mode := api.AuditLogStandard
	result, err := svc.UpdateSettings(context.Background(), UpdateSettingsRequest{
		AuditLogMode: &mode,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.RestartRequired {
		t.Error("audit log mode change should not require restart")
	}
	if database.GetSetting("auditLogMode", "off") != "standard" {
		t.Error("expected auditLogMode to be 'standard'")
	}
}

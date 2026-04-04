package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/thesimonho/warden/api"
)

func TestGetAuditLog(t *testing.T) {
	t.Parallel()

	entries := []api.AuditEntry{
		{Event: "session_start", Category: "session"},
		{Event: "tool_use", Category: "agent"},
	}
	c := newTestServer(t, "GET", "/api/v1/audit", http.StatusOK, entries)

	result, err := c.GetAuditLog(context.Background(), api.AuditFilters{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestGetAuditSummary(t *testing.T) {
	t.Parallel()

	summary := api.AuditSummary{
		TotalSessions: 5,
		TotalToolUses: 42,
	}
	c := newTestServer(t, "GET", "/api/v1/audit/summary", http.StatusOK, summary)

	resp, err := c.GetAuditSummary(context.Background(), api.AuditFilters{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TotalSessions != 5 {
		t.Errorf("expected 5 sessions, got %d", resp.TotalSessions)
	}
	if resp.TotalToolUses != 42 {
		t.Errorf("expected 42 tool uses, got %d", resp.TotalToolUses)
	}
}

func TestGetAuditProjects(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "GET", "/api/v1/audit/projects", http.StatusOK, []string{"project-a", "project-b"})

	result, err := c.GetAuditProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(result))
	}
}

func TestPostAuditEvent(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/audit", http.StatusNoContent, nil)

	err := c.PostAuditEvent(context.Background(), api.PostAuditEventRequest{
		Event:   "page_view",
		Message: "user viewed audit page",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestDeleteAuditEvents(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/audit", http.StatusNoContent, nil)

	err := c.DeleteAuditEvents(context.Background(), api.AuditFilters{
		ProjectID: "abc123def456",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

package client

import (
	"context"
	"net/http"
	"testing"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
)

func TestListAccessItems(t *testing.T) {
	t.Parallel()

	resp := api.AccessItemListResponse{
		Items: []api.AccessItemResponse{
			{Item: access.Item{ID: "git", Label: "Git"}},
			{Item: access.Item{ID: "ssh", Label: "SSH"}},
		},
	}
	c := newTestServer(t, "GET", "/api/v1/access", http.StatusOK, resp)

	result, err := c.ListAccessItems(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result.Items) != 2 {
		t.Fatalf("expected 2 items, got %d", len(result.Items))
	}
	if result.Items[0].ID != "git" {
		t.Errorf("expected first item ID 'git', got %q", result.Items[0].ID)
	}
}

func TestGetAccessItem(t *testing.T) {
	t.Parallel()

	item := api.AccessItemResponse{
		Item: access.Item{ID: "git", Label: "Git"},
	}
	c := newTestServer(t, "GET", "/api/v1/access/git", http.StatusOK, item)

	resp, err := c.GetAccessItem(context.Background(), "git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Label != "Git" {
		t.Errorf("expected label 'Git', got %q", resp.Label)
	}
}

func TestCreateAccessItem(t *testing.T) {
	t.Parallel()

	created := access.Item{ID: "custom-1", Label: "My Token"}
	c := newTestServer(t, "POST", "/api/v1/access", http.StatusCreated, created)

	resp, err := c.CreateAccessItem(context.Background(), api.CreateAccessItemRequest{
		Label:       "My Token",
		Description: "Custom API token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Label != "My Token" {
		t.Errorf("expected label 'My Token', got %q", resp.Label)
	}
}

func TestUpdateAccessItem(t *testing.T) {
	t.Parallel()

	updated := access.Item{ID: "custom-1", Label: "Updated Token"}
	c := newTestServer(t, "PUT", "/api/v1/access/custom-1", http.StatusOK, updated)

	label := "Updated Token"
	resp, err := c.UpdateAccessItem(context.Background(), "custom-1", api.UpdateAccessItemRequest{
		Label: &label,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Label != "Updated Token" {
		t.Errorf("expected label 'Updated Token', got %q", resp.Label)
	}
}

func TestDeleteAccessItem(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/access/custom-1", http.StatusNoContent, nil)

	err := c.DeleteAccessItem(context.Background(), "custom-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestResetAccessItem(t *testing.T) {
	t.Parallel()

	item := access.Item{ID: "git", Label: "Git"}
	c := newTestServer(t, "POST", "/api/v1/access/git/reset", http.StatusOK, item)

	resp, err := c.ResetAccessItem(context.Background(), "git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.ID != "git" {
		t.Errorf("expected ID 'git', got %q", resp.ID)
	}
}

func TestResolveAccessItems(t *testing.T) {
	t.Parallel()

	resolved := api.ResolveAccessItemsResponse{
		Items: []access.ResolvedItem{
			{ID: "git", Label: "Git"},
		},
	}
	c := newTestServer(t, "POST", "/api/v1/access/resolve", http.StatusOK, resolved)

	resp, err := c.ResolveAccessItems(context.Background(), api.ResolveAccessItemsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 resolved item, got %d", len(resp.Items))
	}
}

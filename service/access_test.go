package service

import (
	"testing"

	"github.com/thesimonho/warden/access"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
)

func TestListAccessItems_IncludesBuiltIns(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	items, err := svc.ListAccessItems()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(items) < 2 {
		t.Fatalf("expected at least 2 built-in items, got %d", len(items))
	}

	ids := make(map[string]bool)
	for _, item := range items {
		ids[item.ID] = true
	}
	if !ids[access.BuiltInIDGit] {
		t.Error("expected git built-in item")
	}
	if !ids[access.BuiltInIDSSH] {
		t.Error("expected ssh built-in item")
	}
}

func TestCreateAccessItem(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	item, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label:       "GitHub CLI",
		Description: "Passes GH_TOKEN into the container",
		Credentials: []access.Credential{
			{
				Label:   "GitHub Token",
				Sources: []access.Source{{Type: access.SourceCommand, Value: "echo test-token"}},
				Injections: []access.Injection{
					{Type: access.InjectionEnvVar, Key: "GH_TOKEN"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if item.ID == "" {
		t.Error("expected non-empty ID")
	}
	if item.Label != "GitHub CLI" {
		t.Errorf("expected label 'GitHub CLI', got %q", item.Label)
	}
	if len(item.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(item.Credentials))
	}

	// Verify it appears in the list.
	items, err := svc.ListAccessItems()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	found := false
	for _, i := range items {
		if i.ID == item.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("created item not found in list")
	}
}

func TestCreateAccessItem_ValidationErrors(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	_, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label: "",
	})
	if err == nil {
		t.Error("expected error for empty label")
	}

	_, err = svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label:       "Test",
		Credentials: nil,
	})
	if err == nil {
		t.Error("expected error for empty credentials")
	}
}

func TestUpdateAccessItem_UserItem(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	item, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label:       "Original",
		Description: "Original description",
		Credentials: []access.Credential{
			{
				Label:      "Token",
				Sources:    []access.Source{{Type: access.SourceEnvVar, Value: "MY_TOKEN"}},
				Injections: []access.Injection{{Type: access.InjectionEnvVar, Key: "MY_TOKEN"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	newLabel := "Updated"
	updated, err := svc.UpdateAccessItem(item.ID, api.UpdateAccessItemRequest{
		Label: &newLabel,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if updated.Label != "Updated" {
		t.Errorf("expected label 'Updated', got %q", updated.Label)
	}
}

func TestUpdateAccessItem_BuiltInCreatesOverride(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	newDesc := "Custom SSH description"
	updated, err := svc.UpdateAccessItem(access.BuiltInIDSSH, api.UpdateAccessItemRequest{
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}
	if updated.Description != "Custom SSH description" {
		t.Errorf("expected custom description, got %q", updated.Description)
	}
	if !updated.BuiltIn {
		t.Error("expected BuiltIn=true on modified built-in")
	}

	// Verify the override is returned in the list.
	items, err := svc.ListAccessItems()
	if err != nil {
		t.Fatalf("list error: %v", err)
	}
	for _, item := range items {
		if item.ID == access.BuiltInIDSSH {
			if item.Description != "Custom SSH description" {
				t.Errorf("list should return override, got description %q", item.Description)
			}
			return
		}
	}
	t.Error("SSH item not found in list")
}

func TestResetAccessItem_RestoresDefault(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	// Customize the git built-in.
	newDesc := "Custom Git"
	_, err := svc.UpdateAccessItem(access.BuiltInIDGit, api.UpdateAccessItemRequest{
		Description: &newDesc,
	})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	// Reset to default.
	item, err := svc.ResetAccessItem(access.BuiltInIDGit)
	if err != nil {
		t.Fatalf("reset error: %v", err)
	}

	defaultGit := access.BuiltInGit()
	if item.Description != defaultGit.Description {
		t.Errorf("expected default description %q, got %q", defaultGit.Description, item.Description)
	}

	// Verify the list returns the default.
	resp, err := svc.GetAccessItem(access.BuiltInIDGit)
	if err != nil {
		t.Fatalf("get error: %v", err)
	}
	if resp.Description != defaultGit.Description {
		t.Errorf("get should return default after reset, got %q", resp.Description)
	}
}

func TestResetAccessItem_RejectsNonBuiltIn(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	_, err := svc.ResetAccessItem("custom-id")
	if err == nil {
		t.Error("expected error for non-built-in reset")
	}
}

func TestDeleteAccessItem_RejectsBuiltIn(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	err := svc.DeleteAccessItem(access.BuiltInIDGit)
	if err == nil {
		t.Error("expected error for deleting built-in")
	}
}

func TestDeleteAccessItem_UserItem(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	item, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label:       "Temp",
		Credentials: []access.Credential{{Label: "x", Sources: []access.Source{{Type: access.SourceEnvVar, Value: "X"}}, Injections: []access.Injection{{Type: access.InjectionEnvVar, Key: "X"}}}},
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	if err := svc.DeleteAccessItem(item.ID); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	resp, err := svc.GetAccessItem(item.ID)
	if err == nil && resp != nil {
		t.Error("expected item to be gone after delete")
	}
}

func TestResolveAccessItems_BuiltIn(t *testing.T) {
	t.Parallel()
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	// Resolve git built-in — will resolve if ~/.gitconfig exists on the host.
	resp, err := svc.ResolveAccessItems([]access.Item{*access.BuiltInItemByID(access.BuiltInIDGit)})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 resolved item, got %d", len(resp.Items))
	}
	if resp.Items[0].ID != access.BuiltInIDGit {
		t.Errorf("expected git item, got %q", resp.Items[0].ID)
	}
}

func TestResolveAccessItems_UserItem(t *testing.T) {
	// Cannot use t.Parallel() with t.Setenv.
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: testDB(t)})

	t.Setenv("WARDEN_TEST_ACCESS_TOKEN", "secret123")

	item, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label: "Test Token",
		Credentials: []access.Credential{
			{
				Label:      "Token",
				Sources:    []access.Source{{Type: access.SourceEnvVar, Value: "WARDEN_TEST_ACCESS_TOKEN"}},
				Injections: []access.Injection{{Type: access.InjectionEnvVar, Key: "CONTAINER_TOKEN"}},
			},
		},
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	resp, err := svc.ResolveAccessItems([]access.Item{*item})
	if err != nil {
		t.Fatalf("resolve error: %v", err)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 resolved item, got %d", len(resp.Items))
	}

	cred := resp.Items[0].Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved")
	}
	if cred.Injections[0].Value != "secret123" {
		t.Errorf("expected 'secret123', got %q", cred.Injections[0].Value)
	}
}

func TestAccessItemCRUD_WritesAuditEntries(t *testing.T) {
	t.Parallel()

	store := testDB(t)
	audit := db.NewAuditWriter(store, db.AuditStandard, StandardAuditEvents())
	svc := New(ServiceDeps{Engine: &mockEngine{}, DB: store, Audit: audit})

	// Create — should write access_item_created.
	item, err := svc.CreateAccessItem(api.CreateAccessItemRequest{
		Label:       "Audit Test",
		Credentials: []access.Credential{{Label: "t", Sources: []access.Source{{Type: access.SourceEnvVar, Value: "X"}}, Injections: []access.Injection{{Type: access.InjectionEnvVar, Key: "X"}}}},
	})
	if err != nil {
		t.Fatalf("create error: %v", err)
	}

	// Update — should write access_item_updated.
	newLabel := "Audit Test Updated"
	_, err = svc.UpdateAccessItem(item.ID, api.UpdateAccessItemRequest{Label: &newLabel})
	if err != nil {
		t.Fatalf("update error: %v", err)
	}

	// Delete — should write access_item_deleted.
	if err := svc.DeleteAccessItem(item.ID); err != nil {
		t.Fatalf("delete error: %v", err)
	}

	// Reset built-in — should write access_item_reset.
	newDesc := "Custom"
	_, err = svc.UpdateAccessItem(access.BuiltInIDGit, api.UpdateAccessItemRequest{Description: &newDesc})
	if err != nil {
		t.Fatalf("update built-in error: %v", err)
	}
	_, err = svc.ResetAccessItem(access.BuiltInIDGit)
	if err != nil {
		t.Fatalf("reset error: %v", err)
	}

	// Query audit log for system events.
	entries, err := svc.GetAuditLog(api.AuditFilters{Category: api.AuditCategorySystem})
	if err != nil {
		t.Fatalf("GetAuditLog error: %v", err)
	}

	eventCounts := make(map[string]int)
	for _, e := range entries {
		eventCounts[e.Event]++
	}

	if eventCounts["access_item_created"] != 1 {
		t.Errorf("expected 1 access_item_created event, got %d", eventCounts["access_item_created"])
	}
	if eventCounts["access_item_updated"] != 2 { // user item + built-in override
		t.Errorf("expected 2 access_item_updated events, got %d", eventCounts["access_item_updated"])
	}

	// Verify changedFields is populated on update events.
	for _, e := range entries {
		if e.Event == "access_item_updated" {
			attrs := e.Attrs
			if attrs == nil {
				t.Error("expected attrs on access_item_updated event")
				continue
			}
			if _, ok := attrs["changedFields"]; !ok {
				t.Error("expected changedFields in attrs")
			}
		}
	}
	if eventCounts["access_item_deleted"] != 1 {
		t.Errorf("expected 1 access_item_deleted event, got %d", eventCounts["access_item_deleted"])
	}
	if eventCounts["access_item_reset"] != 1 {
		t.Errorf("expected 1 access_item_reset event, got %d", eventCounts["access_item_reset"])
	}
}

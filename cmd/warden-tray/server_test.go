package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestIsHealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Warden", "1")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	if !client.isHealthy() {
		t.Fatal("expected healthy")
	}
}

func TestIsHealthy_NoHeader(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	if client.isHealthy() {
		t.Fatal("expected unhealthy without X-Warden header")
	}
}

func TestIsHealthy_ServerDown(t *testing.T) {
	client := newServerClient("http://127.0.0.1:0")
	if client.isHealthy() {
		t.Fatal("expected unhealthy for unreachable server")
	}
}

func TestListProjects(t *testing.T) {
	projects := []project{
		{ProjectID: "abc123", Name: "test", AgentType: "claude-code", State: "running"},
		{ProjectID: "def456", Name: "other", AgentType: "codex", State: "exited"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects) //nolint:errcheck
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	got, err := client.listProjects()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 projects, got %d", len(got))
	}
	if got[0].State != "running" {
		t.Errorf("expected first project running, got %q", got[0].State)
	}
}

func TestRunningContainerCount(t *testing.T) {
	projects := []project{
		{State: "running"},
		{State: "exited"},
		{State: "running"},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(projects) //nolint:errcheck
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	count := client.runningContainerCount()
	if count != 2 {
		t.Errorf("expected 2 running, got %d", count)
	}
}

func TestStopProject(t *testing.T) {
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	err := client.stopProject("abc123", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotPath != "/api/v1/projects/abc123/claude-code/stop" {
		t.Errorf("unexpected path: %s", gotPath)
	}
}

func TestWaitForReady_AlreadyUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("X-Warden", "1")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := newServerClient(srv.URL)
	if !client.waitForReady() {
		t.Fatal("expected ready")
	}
}

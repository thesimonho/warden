package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/docker"
	"github.com/thesimonho/warden/service"
)

// newTestServer creates an httptest server that responds to a specific
// method+path with the given status and JSON body.
func newTestServer(t *testing.T, method, path string, status int, body any) *Client {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != nil {
			json.NewEncoder(w).Encode(body) //nolint:errcheck
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return New(srv.URL)
}

func TestListProjects(t *testing.T) {
	t.Parallel()

	projects := []api.ProjectResponse{
		{ID: "abc123def456", Name: "test-project", State: "running"},
	}
	c := newTestServer(t, "GET", "/api/v1/projects", http.StatusOK, projects)

	result, err := c.ListProjects(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 project, got %d", len(result))
	}
	if result[0].Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", result[0].Name)
	}
}

func TestStopProject(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/stop", http.StatusOK, service.ProjectResult{Name: "test", ContainerID: "abc123def456"})

	_, err := c.StopProject(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestListWorktrees(t *testing.T) {
	t.Parallel()

	worktrees := []engine.Worktree{
		{ID: "main", Branch: "main", State: "connected"},
	}
	c := newTestServer(t, "GET", "/api/v1/projects/abc123def456/claude-code/worktrees", http.StatusOK, worktrees)

	result, err := c.ListWorktrees(context.Background(), "abc123def456", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 worktree, got %d", len(result))
	}
}

func TestCreateWorktree(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "POST", "/api/v1/projects/abc123def456/claude-code/worktrees", http.StatusCreated, service.WorktreeResult{WorktreeID: "feature-x", ProjectID: "abc123def456"})

	resp, err := c.CreateWorktree(context.Background(), "abc123def456", "claude-code", api.CreateWorktreeRequest{Name: "feature-x"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.WorktreeID != "feature-x" {
		t.Errorf("expected worktree ID 'feature-x', got %q", resp.WorktreeID)
	}
}

func TestDeleteContainer(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "DELETE", "/api/v1/projects/aabbccddeeff/claude-code/container", http.StatusOK, service.ContainerResult{ContainerID: "abc123def456", Name: "test"})

	_, err := c.DeleteContainer(context.Background(), "aabbccddeeff", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestAPIError(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "GET", "/api/v1/projects", http.StatusInternalServerError, map[string]string{"error": "docker daemon unavailable"})

	_, err := c.ListProjects(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}

	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("expected APIError, got %T: %v", err, err)
	}
	if apiErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", apiErr.StatusCode)
	}
	if apiErr.Message != "docker daemon unavailable" {
		t.Errorf("expected message 'docker daemon unavailable', got %q", apiErr.Message)
	}
}

func TestRemoveProject_EncodesName(t *testing.T) {
	t.Parallel()

	// The path should be URL-encoded.
	c := newTestServer(t, "DELETE", "/api/v1/projects/my%20project/claude-code", http.StatusOK, service.ProjectResult{Name: "my project"})

	_, err := c.RemoveProject(context.Background(), "my project", "claude-code")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// --- Host Utilities ---

func TestGetDefaults(t *testing.T) {
	t.Parallel()

	defaults := service.DefaultsResponse{
		HomeDir:          "/home/user",
		ContainerHomeDir: "/home/warden",
		Mounts: []service.DefaultMount{
			{HostPath: "/home/user/.claude", ContainerPath: "/home/warden/.claude"},
		},
	}
	c := newTestServer(t, "GET", "/api/v1/defaults", http.StatusOK, defaults)

	result, err := c.GetDefaults(context.Background(), "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.HomeDir != "/home/user" {
		t.Errorf("expected HomeDir '/home/user', got %q", result.HomeDir)
	}
	if len(result.Mounts) != 1 {
		t.Fatalf("expected 1 mount, got %d", len(result.Mounts))
	}
}

func TestListDirectories(t *testing.T) {
	t.Parallel()

	entries := []api.DirEntry{
		{Name: "src", Path: "/project/src", IsDir: true},
		{Name: "docs", Path: "/project/docs", IsDir: true},
	}
	c := newTestServer(t, "GET", "/api/v1/filesystem/directories", http.StatusOK, entries)

	result, err := c.ListDirectories(context.Background(), "/project", false)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(result))
	}
}

func TestListRuntimes(t *testing.T) {
	t.Parallel()

	info := docker.Info{Name: docker.Name, Available: true}
	c := newTestServer(t, "GET", "/api/v1/runtimes", http.StatusOK, info)

	result, err := c.ListRuntimes(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Available {
		t.Error("expected docker to be available")
	}
}

// --- SSE Events ---

func TestSubscribeEvents(t *testing.T) {
	t.Parallel()

	// Create an SSE server that sends two events then closes.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/events", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("expected Flusher")
			return
		}

		// Event 1: worktree_state
		_, _ = fmt.Fprintf(w, "event:worktree_state\ndata:{\"containerName\":\"test\"}\n\n")
		flusher.Flush()

		// Event 2: project_state
		_, _ = fmt.Fprintf(w, "event:project_state\ndata:{\"totalCost\":1.23}\n\n")
		flusher.Flush()
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := New(srv.URL)

	ch, unsub, err := c.SubscribeEvents(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer unsub()

	// Read first event.
	event1 := <-ch
	if event1.Event != eventbus.SSEWorktreeState {
		t.Errorf("expected worktree_state, got %q", event1.Event)
	}

	// Read second event.
	event2 := <-ch
	if event2.Event != eventbus.SSEProjectState {
		t.Errorf("expected project_state, got %q", event2.Event)
	}

	// Channel should close after server closes.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed")
	}
}

func TestSubscribeEvents_ServerError(t *testing.T) {
	t.Parallel()

	c := newTestServer(t, "GET", "/api/v1/events", http.StatusInternalServerError, nil)

	_, _, err := c.SubscribeEvents(context.Background())
	if err == nil {
		t.Fatal("expected error for non-200 response")
	}
}

func TestReadSSEStream_ParsesMultipleEvents(t *testing.T) {
	t.Parallel()

	// Directly test the SSE parser with a pipe.
	pr, pw := io.Pipe()
	ch := make(chan eventbus.SSEEvent, 10)

	go readSSEStream(context.Background(), pr, ch)

	// Write SSE-formatted events.
	_, _ = fmt.Fprintf(pw, "event:heartbeat\ndata:{}\n\n")
	_, _ = fmt.Fprintf(pw, "event:worktree_list_changed\ndata:{\"containerName\":\"c1\"}\n\n")
	_ = pw.Close()

	events := make([]eventbus.SSEEvent, 0)
	for e := range ch {
		events = append(events, e)
	}

	if len(events) != 2 {
		t.Fatalf("expected 2 events, got %d", len(events))
	}
	if events[0].Event != eventbus.SSEHeartbeat {
		t.Errorf("expected heartbeat, got %q", events[0].Event)
	}
	if events[1].Event != eventbus.SSEWorktreeListChanged {
		t.Errorf("expected worktree_list_changed, got %q", events[1].Event)
	}
}

func TestReadSSEStream_IgnoresIncompleteEvents(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	ch := make(chan eventbus.SSEEvent, 10)

	go readSSEStream(context.Background(), pr, ch)

	// Event type without data — should be ignored.
	_, _ = fmt.Fprintf(pw, "event:heartbeat\n\n")
	// Data without event type — should be ignored.
	_, _ = fmt.Fprintf(pw, "data:{}\n\n")
	// Complete event.
	_, _ = fmt.Fprintf(pw, "event:project_state\ndata:{\"cost\":1}\n\n")
	_ = pw.Close()

	events := make([]eventbus.SSEEvent, 0)
	for e := range ch {
		events = append(events, e)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 complete event, got %d", len(events))
	}
	if events[0].Event != eventbus.SSEProjectState {
		t.Errorf("expected project_state, got %q", events[0].Event)
	}
}

func TestReadSSEStream_CancelledContext(t *testing.T) {
	t.Parallel()

	pr, pw := io.Pipe()
	ch := make(chan eventbus.SSEEvent, 10)

	ctx, cancel := context.WithCancel(context.Background())

	go readSSEStream(ctx, pr, ch)

	// Send one event.
	_, _ = fmt.Fprintf(pw, "event:heartbeat\ndata:{}\n\n")

	// Read it.
	<-ch

	// Cancel context — stream should stop.
	cancel()
	_ = pw.Close()

	// Channel should close.
	_, ok := <-ch
	if ok {
		t.Error("expected channel to be closed after context cancel")
	}
}

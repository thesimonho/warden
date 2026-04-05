package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
	"github.com/thesimonho/warden/service"
)

// mockEngineClient implements engine.Client for testing.
type mockEngineClient struct {
	projects           []engine.Project
	projectsErr        error
	stopErr            error
	restartErr         error
	containerID        string
	containerErr       error
	deleteContainerErr error
	inspectConfig      *api.ContainerConfig
	inspectErr         error
	recreateID         string
	recreateErr        error
	worktrees          []engine.Worktree
	worktreesErr       error
	connectResp        string
	connectErr         error
	createWorktreeResp string
	createWorktreeErr  error
	disconnectErr      error
	killWorktreeErr    error
	validateValid      bool
	validateMissing    []string
	validateErr        error
}

func (m *mockEngineClient) ListProjects(_ context.Context, _ []string) ([]engine.Project, error) {
	return m.projects, m.projectsErr
}

func (m *mockEngineClient) StopProject(_ context.Context, _ string) error {
	return m.stopErr
}

func (m *mockEngineClient) RestartProject(_ context.Context, _ string, _ []api.Mount) error {
	return m.restartErr
}

func (m *mockEngineClient) CreateContainer(_ context.Context, _ api.CreateContainerRequest) (string, error) {
	return m.containerID, m.containerErr
}

func (m *mockEngineClient) DeleteContainer(_ context.Context, _ string) error {
	return m.deleteContainerErr
}

func (m *mockEngineClient) CleanupEventDir(_ string) {}

func (m *mockEngineClient) InspectContainer(_ context.Context, _ string) (*api.ContainerConfig, error) {
	return m.inspectConfig, m.inspectErr
}

func (m *mockEngineClient) RenameContainer(_ context.Context, _ string, _ string) error {
	return nil
}

func (m *mockEngineClient) ReloadAllowedDomains(_ context.Context, _ string, _ []string) error {
	return nil
}

func (m *mockEngineClient) RecreateContainer(_ context.Context, _ string, _ api.CreateContainerRequest) (string, error) {
	return m.recreateID, m.recreateErr
}

func (m *mockEngineClient) ListWorktrees(_ context.Context, _ string, _ bool) ([]engine.Worktree, error) {
	return m.worktrees, m.worktreesErr
}

func (m *mockEngineClient) CreateWorktree(_ context.Context, _, _ string, _ bool) (string, error) {
	return m.createWorktreeResp, m.createWorktreeErr
}

func (m *mockEngineClient) ConnectTerminal(_ context.Context, _, _ string, _ bool) (string, error) {
	return m.connectResp, m.connectErr
}

func (m *mockEngineClient) DisconnectTerminal(_ context.Context, _, _ string) error {
	return m.disconnectErr
}

func (m *mockEngineClient) KillWorktreeProcess(_ context.Context, _, _ string) error {
	return m.killWorktreeErr
}

func (m *mockEngineClient) ResetWorktree(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockEngineClient) RemoveWorktree(_ context.Context, _, _ string) error {
	return nil
}

func (m *mockEngineClient) CleanupOrphanedWorktrees(_ context.Context, _ string) ([]string, error) {
	return nil, nil
}

func (m *mockEngineClient) ValidateInfrastructure(_ context.Context, _ string) (bool, []string, error) {
	return m.validateValid, m.validateMissing, m.validateErr
}

func (m *mockEngineClient) ReadAgentStatus(_ context.Context, _ string) (map[string]*agent.Status, error) {
	return nil, nil
}

func (m *mockEngineClient) IsEstimatedCost(_ context.Context, _ string) bool {
	return false
}

func (m *mockEngineClient) ReadAgentCostAndBillingType(_ context.Context, _, _ string) (*engine.AgentCostResult, error) {
	return &engine.AgentCostResult{}, nil
}

func (m *mockEngineClient) GetWorktreeDiff(_ context.Context, _, _ string) (*api.DiffResponse, error) {
	return nil, nil
}

func (m *mockEngineClient) CopyFileToContainer(_ context.Context, _, _, _ string, _ io.Reader, _ int64) error {
	return nil
}

func (m *mockEngineClient) ContainerStartupHealth(_ context.Context, _ string) (*engine.ContainerHealth, error) {
	return &engine.ContainerHealth{}, nil
}

// testDB returns a fresh in-memory SQLite store for tests.
func testDB(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("failed to create test db: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// testProjectID is the deterministic project ID for /home/user/project.
var testProjectID, _ = engine.ProjectID("/home/user/project")

// testContainerID is the Docker container ID used in test fixtures.
const testContainerID = "abc123def456"

// insertTestProject inserts a project row with the standard test container ID
// and returns the project ID.
func insertTestProject(t *testing.T, database *db.Store) string {
	t.Helper()
	if err := database.InsertProject(db.ProjectRow{
		ProjectID:   testProjectID,
		Name:        "test-project",
		HostPath:    "/home/user/project",
		ContainerID: testContainerID,
	}); err != nil {
		t.Fatalf("failed to insert test project: %v", err)
	}
	return testProjectID
}

func TestHandleListProjects(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "test-project", State: "running", Status: "Up 2 hours"},
		},
	}

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var projects []engine.Project
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}

	if projects[0].Name != "test-project" {
		t.Errorf("expected name 'test-project', got %q", projects[0].Name)
	}
}

func TestHandleListProjects_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		projectsErr: errors.New("docker daemon unavailable"),
	}

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleAddProject(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"my-project","projectPath":"/home/user/my-project"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify by decoding the response to get the computed project ID.
	var result api.ProjectResult
	if err := json.NewDecoder(rec.Body).Decode(&result); err == nil && result.ProjectID != "" {
		has, _ := database.HasProject(result.ProjectID, "claude-code")
		if !has {
			t.Error("expected project to be added to database")
		}
	}
}

func TestHandleAddProject_InvalidName(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	body := strings.NewReader(`{"name":"../../etc/passwd"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleRemoveProject(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+pid+"/claude-code", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	has, _ := database.HasProject(pid, "claude-code")
	if has {
		t.Error("expected project to be removed from database")
	}
}

func TestHandleStopProject(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleStopProject_InvalidID(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/INVALID_ID!/claude-code/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleStopProject_DockerError(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		stopErr: errors.New("container not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/stop", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleRestartProject(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/restart", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleListWorktrees(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktrees: []engine.Worktree{
			{ID: "main", ProjectID: testContainerID, Path: "/project", Branch: "main", State: "connected"},
			{ID: "feature-x", ProjectID: testContainerID, Path: "/project/.claude/worktrees/feature-x", Branch: "feature-x", State: "disconnected"},
		},
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var worktrees []engine.Worktree
	if err := json.NewDecoder(rec.Body).Decode(&worktrees); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}
	if worktrees[0].ID != "main" {
		t.Errorf("expected first worktree 'main', got %q", worktrees[0].ID)
	}
}

func TestHandleListWorktrees_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktreesErr: errors.New("container not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleCreateWorktree(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		createWorktreeResp: "feature-y",
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"feature-y"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}

	var resp service.WorktreeResult
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if resp.WorktreeID != "feature-y" {
		t.Errorf("unexpected worktree ID: %s", resp.WorktreeID)
	}
}

func TestHandleCreateWorktree_MissingName(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleCreateWorktree_InvalidBody(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`not json`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleConnectTerminal(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		connectResp: "main",
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/main/connect", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleConnectTerminal_InvalidWorktreeID(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/../connect", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	// The path traversal won't match the route pattern, so it returns 405 or the handler rejects it
	if rec.Code == http.StatusCreated {
		t.Fatal("expected non-201 for invalid worktree ID")
	}
}

func TestHandleDisconnectTerminal(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/main/disconnect", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDisconnectTerminal_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		disconnectErr: errors.New("terminal not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/main/disconnect", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleKillWorktreeProcess(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/main/kill", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleKillWorktreeProcess_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		killWorktreeErr: errors.New("process not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/worktrees/main/kill", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleCreateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		containerID: "abc123def456",
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"my-project","image":"claude-project-dev","projectPath":"/home/user/project"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/container", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleCreateContainer_MissingProjectPath(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"my-project","image":"claude-project-dev"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/v1/projects/"+pid+"/claude-code/container", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleListDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "subdir"), 0o755); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: &mockEngineClient{}, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/directories?path="+dir, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var dirs []struct {
		Name string `json:"name"`
		Path string `json:"path"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&dirs); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(dirs) != 1 || dirs[0].Name != "subdir" {
		t.Errorf("unexpected dirs: %v", dirs)
	}
}

func TestHandleListDirectories_MissingPath(t *testing.T) {
	t.Parallel()

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: &mockEngineClient{}, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/filesystem/directories", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestIsValidContainerID(t *testing.T) {
	t.Parallel()

	validIDs := []string{
		"abc123def456",
		"abc123def456abc123def456abc123def456abc123def456abc123def456abcd",
	}
	for _, id := range validIDs {
		if !isValidContainerID(id) {
			t.Errorf("expected %q to be valid", id)
		}
	}

	invalidIDs := []string{
		"",
		"short",
		"abc123def45",
		"ABC123DEF456",
		"abc123def456!",
		"../etc/passwd",
	}
	for _, id := range invalidIDs {
		if isValidContainerID(id) {
			t.Errorf("expected %q to be invalid", id)
		}
	}
}

func TestIsValidContainerName(t *testing.T) {
	t.Parallel()

	validNames := []string{
		"my-project",
		"claude-webapp",
		"project_1",
		"a",
		"MyProject",
	}
	for _, name := range validNames {
		if !isValidContainerName(name) {
			t.Errorf("expected %q to be valid", name)
		}
	}

	invalidNames := []string{
		"",
		"../etc/passwd",
		"-starts-with-dash",
		"has spaces",
		"has/slash",
	}
	for _, name := range invalidNames {
		if isValidContainerName(name) {
			t.Errorf("expected %q to be invalid", name)
		}
	}
}

func TestHandleDeleteContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+pid+"/claude-code/container", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleDeleteContainer_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		deleteContainerErr: errors.New("container not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/"+pid+"/claude-code/container", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleDeleteContainer_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodDelete, "/api/v1/projects/aabbccddeeff/claude-code/container", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

func TestHandleInspectContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		inspectConfig: &api.ContainerConfig{
			Name:        "my-project",
			Image:       "ghcr.io/thesimonho/warden:latest",
			ProjectPath: "/home/user/project",
		},
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/container/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var cfg api.ContainerConfig
	if err := json.NewDecoder(rec.Body).Decode(&cfg); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if cfg.Name != "my-project" {
		t.Errorf("expected name 'my-project', got %q", cfg.Name)
	}
}

func TestHandleInspectContainer_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		inspectErr: errors.New("not found"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/container/config", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleUpdateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		recreateID: "def456abc123",
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"my-project","image":"warden:latest","projectPath":"/home/user/project"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+pid+"/claude-code/container", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestHandleUpdateContainer_MissingProjectPath(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	body := strings.NewReader(`{"name":"my-project","image":"warden:latest"}`)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/projects/"+pid+"/claude-code/container", body)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestHandleValidateContainer(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		validateValid:   true,
		validateMissing: nil,
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/container/validate", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result["valid"] != true {
		t.Errorf("expected valid=true, got %v", result["valid"])
	}
}

func TestHandleValidateContainer_MissingInfrastructure(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		validateValid:   false,
		validateMissing: []string{"/usr/local/bin/ttyd", "/usr/local/bin/create-terminal.sh"},
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/container/validate", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}

	var result struct {
		Valid   bool     `json:"valid"`
		Missing []string `json:"missing"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&result); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if result.Valid {
		t.Error("expected valid=false")
	}
	if len(result.Missing) != 2 {
		t.Errorf("expected 2 missing items, got %d", len(result.Missing))
	}
}

func TestHandleValidateContainer_Error(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		validateErr: errors.New("container not running"),
	}
	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/container/validate", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestHandleValidateContainer_NotFound(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{}
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/aabbccddeeff/claude-code/container/validate", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}

// ---------------------------------------------------------------------------
// Event store overlay tests
// ---------------------------------------------------------------------------

func TestHandleListProjects_OverlaysCostFromDB(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running"},
		},
	}

	database := testDB(t)
	_ = database.InsertProject(db.ProjectRow{ProjectID: "my-project", Name: "my-project", HostPath: "/test/my-project"})
	_ = database.UpsertSessionCost("my-project", "claude-code", "session-abc", 4.56, false)

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var projects []engine.Project
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(projects) != 1 {
		t.Fatalf("expected 1 project, got %d", len(projects))
	}
	if projects[0].TotalCost != 4.56 {
		t.Errorf("expected totalCost 4.56, got %f", projects[0].TotalCost)
	}
}

func TestHandleListProjects_NilStoreIsNoOp(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		projects: []engine.Project{
			{ID: "abc123def456", Name: "my-project", State: "running", TotalCost: 0},
		},
	}

	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: testDB(t)}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var projects []engine.Project
	if err := json.NewDecoder(rec.Body).Decode(&projects); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if projects[0].TotalCost != 0 {
		t.Errorf("expected totalCost 0 with nil store, got %f", projects[0].TotalCost)
	}
}

func TestHandleListWorktrees_OverlaysAttentionFromStore(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktrees: []engine.Worktree{
			{ID: "main", ProjectID: testContainerID, State: engine.WorktreeStateConnected},
			{ID: "feature-x", ProjectID: testContainerID, State: engine.WorktreeStateStopped},
		},
		inspectConfig: &api.ContainerConfig{Name: "test-project"},
	}

	store := eventbus.NewStore(nil, nil)
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Data:          []byte(`{"notificationType":"permission_prompt"}`),
		Timestamp:     time.Now(),
	})

	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var worktrees []engine.Worktree
	if err := json.NewDecoder(rec.Body).Decode(&worktrees); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	// Connected worktree should have attention overlaid
	if !worktrees[0].NeedsInput {
		t.Error("expected main worktree needsInput=true from store")
	}
	if worktrees[0].NotificationType != "permission_prompt" {
		t.Errorf("expected notificationType 'permission_prompt', got %q", worktrees[0].NotificationType)
	}

	// Disconnected worktree should NOT have attention overlaid
	if worktrees[1].NeedsInput {
		t.Error("expected disconnected worktree needsInput=false (skipped)")
	}
}

func TestHandleListWorktrees_AttentionClearOverlay(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktrees: []engine.Worktree{
			{
				ID:               "main",
				ProjectID:        testContainerID,
				State:            engine.WorktreeStateConnected,
				NeedsInput:       true,
				NotificationType: "permission_prompt",
			},
		},
		inspectConfig: &api.ContainerConfig{Name: "test-project"},
	}

	store := eventbus.NewStore(nil, nil)
	// First set attention, then clear it
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Data:          []byte(`{"notificationType":"permission_prompt"}`),
		Timestamp:     time.Now(),
	})
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttentionClear,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var worktrees []engine.Worktree
	if err := json.NewDecoder(rec.Body).Decode(&worktrees); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Store says attention is cleared — should override the engine's stale data
	if worktrees[0].NeedsInput {
		t.Error("expected needsInput=false after attention_clear event")
	}
	if worktrees[0].NotificationType != "" {
		t.Errorf("expected empty notificationType after clear, got %q", worktrees[0].NotificationType)
	}
}

func TestHandleListWorktrees_OverlayWorksWithoutInspect(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktrees: []engine.Worktree{
			{ID: "main", ProjectID: testContainerID, State: engine.WorktreeStateConnected},
		},
		// Inspect is no longer needed — container name comes from the project row.
		inspectErr: errors.New("container not found"),
	}

	store := eventbus.NewStore(nil, nil)
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventAttention,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Data:          []byte(`{"notificationType":"permission_prompt"}`),
		Timestamp:     time.Now(),
	})

	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var worktrees []engine.Worktree
	if err := json.NewDecoder(rec.Body).Decode(&worktrees); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Container name is resolved from the DB row, so the overlay applies
	// even when Docker inspect fails.
	if !worktrees[0].NeedsInput {
		t.Error("expected needsInput=true — overlay should work via project row")
	}
}

func TestHandleListWorktrees_SessionEndTransitionsToShell(t *testing.T) {
	t.Parallel()

	mock := &mockEngineClient{
		worktrees: []engine.Worktree{
			{ID: "main", ProjectID: testContainerID, State: engine.WorktreeStateConnected},
		},
		inspectConfig: &api.ContainerConfig{Name: "test-project"},
	}

	store := eventbus.NewStore(nil, nil)
	// Start session, then end it.
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventSessionStart,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(eventbus.ContainerEvent{
		Type:          eventbus.EventSessionEnd,
		ContainerName: "test-project",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	database := testDB(t)
	pid := insertTestProject(t, database)
	mux := http.NewServeMux()
	registerAPIRoutes(mux, service.New(service.ServiceDeps{DockerAvailable: true, Engine: mock, DB: database, Store: store}), nil, nil)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/projects/"+pid+"/claude-code/worktrees", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)

	var worktrees []engine.Worktree
	if err := json.NewDecoder(rec.Body).Decode(&worktrees); err != nil {
		t.Fatalf("decode error: %v", err)
	}

	// Engine says "connected" but store says session ended → should be "shell"
	if worktrees[0].State != engine.WorktreeStateShell {
		t.Errorf("expected state 'shell' after session end, got %q", worktrees[0].State)
	}
}

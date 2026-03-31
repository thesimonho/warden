package service

import (
	"context"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
)

// mockEngine implements engine.Client for service-level testing.
type mockEngine struct {
	projects           []engine.Project
	projectsErr        error
	stopErr            error
	restartErr         error
	containerID        string
	containerErr       error
	deleteContainerErr error
	inspectConfig      *engine.ContainerConfig
	inspectErr         error
	renameErr          error
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
	removeWorktreeErr  error
	cleanupRemoved     []string
	cleanupErr         error
	validateValid      bool
	validateMissing    []string
	validateErr        error
	// Call tracking for assertions.
	killedWorktrees []string
	stopCalled      bool
}

func (m *mockEngine) ListProjects(_ context.Context, _ []string) ([]engine.Project, error) {
	return m.projects, m.projectsErr
}

func (m *mockEngine) StopProject(_ context.Context, _ string) error {
	m.stopCalled = true
	return m.stopErr
}

func (m *mockEngine) RestartProject(_ context.Context, _ string, _ []engine.Mount) error {
	return m.restartErr
}

func (m *mockEngine) CreateContainer(_ context.Context, _ engine.CreateContainerRequest) (string, error) {
	return m.containerID, m.containerErr
}

func (m *mockEngine) DeleteContainer(_ context.Context, _ string) error {
	return m.deleteContainerErr
}

func (m *mockEngine) CleanupEventDir(_ string) {}

func (m *mockEngine) InspectContainer(_ context.Context, _ string) (*engine.ContainerConfig, error) {
	return m.inspectConfig, m.inspectErr
}

func (m *mockEngine) RenameContainer(_ context.Context, _ string, _ string) error {
	return m.renameErr
}

func (m *mockEngine) RecreateContainer(_ context.Context, _ string, _ engine.CreateContainerRequest) (string, error) {
	return m.recreateID, m.recreateErr
}

func (m *mockEngine) ListWorktrees(_ context.Context, _ string, _ bool) ([]engine.Worktree, error) {
	return m.worktrees, m.worktreesErr
}

func (m *mockEngine) CreateWorktree(_ context.Context, _, _ string, _ bool) (string, error) {
	return m.createWorktreeResp, m.createWorktreeErr
}

func (m *mockEngine) ConnectTerminal(_ context.Context, _, _ string, _ bool) (string, error) {
	return m.connectResp, m.connectErr
}

func (m *mockEngine) DisconnectTerminal(_ context.Context, _, _ string) error {
	return m.disconnectErr
}

func (m *mockEngine) KillWorktreeProcess(_ context.Context, _, wtID string) error {
	m.killedWorktrees = append(m.killedWorktrees, wtID)
	return m.killWorktreeErr
}

func (m *mockEngine) RemoveWorktree(_ context.Context, _, _ string) error {
	return m.removeWorktreeErr
}

func (m *mockEngine) CleanupOrphanedWorktrees(_ context.Context, _ string) ([]string, error) {
	return m.cleanupRemoved, m.cleanupErr
}

func (m *mockEngine) ValidateInfrastructure(_ context.Context, _ string) (bool, []string, error) {
	return m.validateValid, m.validateMissing, m.validateErr
}

func (m *mockEngine) ReadAgentStatus(_ context.Context, _ string) (map[string]*agent.Status, error) {
	return nil, nil
}

func (m *mockEngine) IsEstimatedCost(_ context.Context, _ string) bool {
	return false
}

func (m *mockEngine) ReadAgentCostAndBillingType(_ context.Context, _, _ string) (*engine.AgentCostResult, error) {
	return &engine.AgentCostResult{}, nil
}

func (m *mockEngine) GetWorktreeDiff(_ context.Context, _, _ string) (*api.DiffResponse, error) {
	return nil, nil
}

func (m *mockEngine) ContainerStartupHealth(_ context.Context, _ string) (*engine.ContainerHealth, error) {
	return &engine.ContainerHealth{}, nil
}

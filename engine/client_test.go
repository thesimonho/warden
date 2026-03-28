package engine

import (
	"context"
	"testing"

	"github.com/docker/docker/api/types/container"
)

// ---------------------------------------------------------------------------
// String utilities
// ---------------------------------------------------------------------------

func TestTruncateImage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{name: "named image", image: "ubuntu:24.04", expected: "ubuntu:24.04"},
		{name: "sha256 digest", image: "sha256:abc123def456789abcdef0123456789", expected: "sha256:abc123def456"},
		{name: "short sha256", image: "sha256:abc", expected: "sha256:abc"},
		{name: "empty", image: "", expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := truncateImage(tc.image)
			if got != tc.expected {
				t.Errorf("truncateImage(%q) = %q, want %q", tc.image, got, tc.expected)
			}
		})
	}
}

func TestBuildOSLabel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "name and version",
			labels:   map[string]string{"org.opencontainers.image.ref.name": "ubuntu", "org.opencontainers.image.version": "24.04"},
			expected: "ubuntu 24.04",
		},
		{
			name:     "name only",
			labels:   map[string]string{"org.opencontainers.image.ref.name": "ubuntu"},
			expected: "ubuntu",
		},
		{
			name:     "version only",
			labels:   map[string]string{"org.opencontainers.image.version": "24.04"},
			expected: "24.04",
		},
		{
			name:     "neither",
			labels:   map[string]string{},
			expected: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := buildOSLabel(tc.labels)
			if got != tc.expected {
				t.Errorf("buildOSLabel() = %q, want %q", got, tc.expected)
			}
		})
	}
}

func TestContainerName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		names    []string
		expected string
	}{
		{name: "with slash", names: []string{"/mycontainer"}, expected: "mycontainer"},
		{name: "no slash", names: []string{"mycontainer"}, expected: "mycontainer"},
		{name: "empty", names: []string{}, expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := containerName(tc.names)
			if got != tc.expected {
				t.Errorf("containerName(%v) = %q, want %q", tc.names, got, tc.expected)
			}
		})
	}
}

func TestFindHostPort(t *testing.T) {
	t.Parallel()

	ports := []container.Port{
		{PrivatePort: 22, PublicPort: 2222},
		{PrivatePort: 7682, PublicPort: 7702},
		{PrivatePort: 80, PublicPort: 0},
	}

	tests := []struct {
		name          string
		containerPort uint16
		expected      string
	}{
		{name: "found", containerPort: 7682, expected: "7702"},
		{name: "ssh port", containerPort: 22, expected: "2222"},
		{name: "no public port", containerPort: 80, expected: ""},
		{name: "not mapped", containerPort: 443, expected: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := findHostPort(ports, tc.containerPort)
			if got != tc.expected {
				t.Errorf("findHostPort(ports, %d) = %q, want %q", tc.containerPort, got, tc.expected)
			}
		})
	}
}

func TestProjectMountPaths(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mounts     []container.MountPoint
		wantSource string
		wantDest   string
	}{
		{
			name: "modern workspace mount",
			mounts: []container.MountPoint{
				{Source: "/home/user/project", Destination: "/home/dev/test-project"},
			},
			wantSource: "/home/user/project",
			wantDest:   "/home/dev/test-project",
		},
		{
			name: "legacy /project mount",
			mounts: []container.MountPoint{
				{Source: "/home/user/project", Destination: "/project"},
			},
			wantSource: "/home/user/project",
			wantDest:   "/project",
		},
		{
			name: "no project mount",
			mounts: []container.MountPoint{
				{Source: "/data", Destination: "/var/data"},
			},
			wantSource: "",
			wantDest:   "",
		},
		{name: "empty mounts", mounts: nil, wantSource: "", wantDest: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotSource, gotDest := projectMountPaths("test-project", tc.mounts)
			if gotSource != tc.wantSource {
				t.Errorf("projectMountPaths() source = %q, want %q", gotSource, tc.wantSource)
			}
			if gotDest != tc.wantDest {
				t.Errorf("projectMountPaths() dest = %q, want %q", gotDest, tc.wantDest)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Exec-based methods (using mock API)
// ---------------------------------------------------------------------------

func TestCheckIsGitRepo_True(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", "true\n")

	dc := newTestClient(mock)
	result := dc.checkIsGitRepo(context.Background(), "ctr-git")

	if !result {
		t.Error("expected checkIsGitRepo to return true")
	}

	// Verify git command runs directly, not via su.
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == "su" {
			t.Errorf("checkIsGitRepo uses 'su' command: %v", call.Cmd)
		}
	}
}

func TestCheckIsGitRepo_False(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", "")

	dc := newTestClient(mock)
	result := dc.checkIsGitRepo(context.Background(), "ctr-nogit")

	if result {
		t.Error("expected checkIsGitRepo to return false for empty output")
	}
}

func TestCheckIsGitRepo_CachesResult(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("git", "true\n")

	dc := newTestClient(mock)

	// First call should exec.
	dc.checkIsGitRepo(context.Background(), "ctr-cache")
	// Second call should use cache.
	dc.checkIsGitRepo(context.Background(), "ctr-cache")

	gitCalls := 0
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == "git" {
			gitCalls++
		}
	}
	if gitCalls != 1 {
		t.Errorf("expected 1 git exec call (cached), got %d", gitCalls)
	}
}

func TestCheckClaudeStatus_Working(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("pgrep", "12345\n")

	dc := newTestClient(mock)
	status := dc.checkClaudeStatus(context.Background(), "ctr-claude")

	if status != ClaudeStatusWorking {
		t.Errorf("expected ClaudeStatusWorking, got %q", status)
	}
}

func TestCheckClaudeStatus_Idle(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("pgrep", "")

	dc := newTestClient(mock)
	status := dc.checkClaudeStatus(context.Background(), "ctr-idle")

	if status != ClaudeStatusIdle {
		t.Errorf("expected ClaudeStatusIdle, got %q", status)
	}
}

func TestValidateInfrastructure_AllPresent(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// All binaries exist — test -x succeeds, no echo output.
	mock.onCmd("test", "")

	dc := newTestClient(mock)
	valid, missing, err := dc.ValidateInfrastructure(context.Background(), "ctr-valid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !valid {
		t.Errorf("expected valid=true, got false (missing: %v)", missing)
	}
	if len(missing) != 0 {
		t.Errorf("expected no missing binaries, got %v", missing)
	}
}

func TestValidateInfrastructure_MissingBinaries(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// ttyd and create-terminal.sh are missing.
	mock.onCmd("test", "/usr/local/bin/ttyd\n/usr/local/bin/create-terminal.sh\n")

	dc := newTestClient(mock)
	valid, missing, err := dc.ValidateInfrastructure(context.Background(), "ctr-missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if valid {
		t.Error("expected valid=false for missing binaries")
	}
	if len(missing) != 2 {
		t.Fatalf("expected 2 missing binaries, got %d: %v", len(missing), missing)
	}
}

func TestConnectTerminal_FreshSession(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// isAbducoSessionAlive returns false (abduco not running).
	mock.onCmd("pgrep", "0\n")
	// create-terminal.sh returns JSON.
	mock.onCmd(createTerminalScript, `{"worktreeId":"feature-x"}`)

	dc := newTestClient(mock)
	resp, err := dc.ConnectTerminal(context.Background(), "ctr-connect", "feature-x", false)
	if err != nil {
		t.Fatalf("ConnectTerminal failed: %v", err)
	}

	if resp != "feature-x" {
		t.Errorf("expected worktreeId 'feature-x', got %q", resp)
	}

	// Verify the create script was called with the worktree ID.
	var createCall *execCall
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == createTerminalScript {
			createCall = &call
			break
		}
	}
	if createCall == nil {
		t.Fatal("expected create-terminal.sh to be called")
		return // unreachable — staticcheck SA5011
	}
	if len(createCall.Cmd) < 2 || createCall.Cmd[1] != "feature-x" {
		t.Errorf("expected create-terminal.sh feature-x, got %v", createCall.Cmd)
	}
	// Without User: "dev", Docker exec defaults to root which doesn't have
	// ~/.local/bin in PATH (where Claude Code is installed).
	if createCall.User != containerUser {
		t.Errorf("expected exec User %q, got %q", containerUser, createCall.User)
	}
}

func TestConnectTerminal_SkipPermissions(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("pgrep", "0\n")
	mock.onCmd(createTerminalScript, `{"worktreeId":"main"}`)

	dc := newTestClient(mock)
	// Pass skipPermissions=true directly.
	_, err := dc.ConnectTerminal(context.Background(), "ctr-skip", "main", true)
	if err != nil {
		t.Fatalf("ConnectTerminal failed: %v", err)
	}

	var createCall *execCall
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == createTerminalScript {
			createCall = &call
			break
		}
	}
	if createCall == nil {
		t.Fatal("expected create-terminal.sh to be called")
		return // unreachable — staticcheck SA5011
	}
	if len(createCall.Cmd) < 3 || createCall.Cmd[2] != "--skip-permissions" {
		t.Errorf("expected --skip-permissions flag, got %v", createCall.Cmd)
	}
}

func TestConnectTerminal_ReconnectBackground(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	// isAbducoSessionAlive returns true.
	mock.onCmd("pgrep", "1\n")

	dc := newTestClient(mock)
	resp, err := dc.ConnectTerminal(context.Background(), "ctr-bg", "bg-task", false)
	if err != nil {
		t.Fatalf("ConnectTerminal failed: %v", err)
	}

	if resp != "bg-task" {
		t.Errorf("expected worktreeId 'bg-task', got %q", resp)
	}

	// Verify create script was NOT called (background reconnect returns early).
	for _, call := range mock.getCalls() {
		if len(call.Cmd) > 0 && call.Cmd[0] == createTerminalScript {
			t.Error("create-terminal.sh should not be called when abduco is alive")
		}
	}
}

func TestConnectTerminal_InvalidWorktreeID(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	dc := newTestClient(mock)

	_, err := dc.ConnectTerminal(context.Background(), "ctr-bad", "../../../etc/passwd", false)
	if err == nil {
		t.Fatal("expected error for invalid worktree ID")
	}
}

func TestCreateWorktree_DelegatesToConnect(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	mock.onCmd("pgrep", "0\n")
	mock.onCmd(createTerminalScript, `{"worktreeId":"new-wt"}`)

	dc := newTestClient(mock)
	resp, err := dc.CreateWorktree(context.Background(), "ctr-create", "new-wt", false)
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	if resp != "new-wt" {
		t.Errorf("expected worktreeId 'new-wt', got %q", resp)
	}
}

func TestCreateWorktree_InvalidName(t *testing.T) {
	t.Parallel()

	mock := newExecMockAPI()
	dc := newTestClient(mock)

	_, err := dc.CreateWorktree(context.Background(), "ctr-bad", "-invalid", false)
	if err == nil {
		t.Fatal("expected error for invalid worktree name")
	}
}

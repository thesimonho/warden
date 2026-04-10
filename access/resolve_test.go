package access

import (
	"net"
	"os"
	"path/filepath"
	"testing"
)

func TestDetect_EnvVarSource(t *testing.T) {
	t.Setenv("TEST_WARDEN_TOKEN", "secret")

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Token",
				Sources: []Source{{Type: SourceEnvVar, Value: "TEST_WARDEN_TOKEN"}},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected item to be available")
	}
	if !result.Credentials[0].Available {
		t.Fatal("expected credential to be available")
	}
	if result.Credentials[0].SourceMatched != "env $TEST_WARDEN_TOKEN" {
		t.Errorf("unexpected source matched: %s", result.Credentials[0].SourceMatched)
	}
}

func TestDetect_EnvVarSource_Missing(t *testing.T) {
	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Token",
				Sources: []Source{{Type: SourceEnvVar, Value: "DEFINITELY_NOT_SET_WARDEN_TEST"}},
			},
		},
	}

	result := Detect(item, nil)
	if result.Available {
		t.Fatal("expected item to be unavailable")
	}
}

func TestDetect_FileSource(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config")
	if err := os.WriteFile(configFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Config",
				Sources: []Source{{Type: SourceFilePath, Value: configFile}},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected item to be available")
	}
}

func TestDetect_FileSource_FallbackOrder(t *testing.T) {
	dir := t.TempDir()
	// First source doesn't exist, second does.
	secondFile := filepath.Join(dir, "fallback")
	if err := os.WriteFile(secondFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label: "Config",
				Sources: []Source{
					{Type: SourceFilePath, Value: filepath.Join(dir, "primary")},
					{Type: SourceFilePath, Value: secondFile},
				},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected item to be available via fallback")
	}
	if result.Credentials[0].SourceMatched != "file "+secondFile {
		t.Errorf("expected fallback source, got: %s", result.Credentials[0].SourceMatched)
	}
}

func TestDetect_SocketSource(t *testing.T) {
	// Create a temp env var pointing to a real socket would be complex.
	// Instead, test that a missing socket returns unavailable.
	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: "/tmp/nonexistent-test-socket.sock"}},
			},
		},
	}

	result := Detect(item, nil)
	if result.Available {
		t.Fatal("expected item to be unavailable for missing socket")
	}
}

func TestDetect_PartialAvailability(t *testing.T) {
	t.Setenv("TEST_WARDEN_PARTIAL", "yes")

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Available",
				Sources: []Source{{Type: SourceEnvVar, Value: "TEST_WARDEN_PARTIAL"}},
			},
			{
				Label:   "Missing",
				Sources: []Source{{Type: SourceEnvVar, Value: "DEFINITELY_NOT_SET_WARDEN_TEST"}},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected item to be available (partial)")
	}
	if !result.Credentials[0].Available {
		t.Fatal("expected first credential to be available")
	}
	if result.Credentials[1].Available {
		t.Fatal("expected second credential to be unavailable")
	}
}

func TestResolve_EnvVarToEnvVar(t *testing.T) {
	t.Setenv("TEST_WARDEN_RESOLVE", "my-token")

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Token",
				Sources: []Source{{Type: SourceEnvVar, Value: "TEST_WARDEN_RESOLVE"}},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "CONTAINER_TOKEN"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Credentials) != 1 {
		t.Fatalf("expected 1 credential, got %d", len(result.Credentials))
	}
	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved")
	}
	if len(cred.Injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(cred.Injections))
	}
	inj := cred.Injections[0]
	if inj.Type != InjectionEnvVar {
		t.Errorf("expected env injection, got %s", inj.Type)
	}
	if inj.Key != "CONTAINER_TOKEN" {
		t.Errorf("unexpected key: %s", inj.Key)
	}
	if inj.Value != "my-token" {
		t.Errorf("unexpected value: %s", inj.Value)
	}
}

func TestResolve_FileToMount(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, ".gitconfig")
	if err := os.WriteFile(configFile, []byte("[user]\nname = Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Git Config",
				Sources: []Source{{Type: SourceFilePath, Value: configFile}},
				Injections: []Injection{
					{Type: InjectionMountFile, Key: "/home/warden/.gitconfig.host", ReadOnly: true},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved")
	}
	inj := cred.Injections[0]
	if inj.Type != InjectionMountFile {
		t.Errorf("expected mount_file injection, got %s", inj.Type)
	}
	if inj.Value != configFile {
		t.Errorf("expected host path %s, got %s", configFile, inj.Value)
	}
	if !inj.ReadOnly {
		t.Error("expected read-only mount")
	}
}

func TestResolve_CommandToEnvVar(t *testing.T) {
	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Echo Token",
				Sources: []Source{{Type: SourceCommand, Value: "echo hello-world"}},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "MY_TOKEN"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved")
	}
	if cred.Injections[0].Value != "hello-world" {
		t.Errorf("expected 'hello-world', got %q", cred.Injections[0].Value)
	}
}

func TestResolve_StaticValueOverride(t *testing.T) {
	t.Setenv("TEST_WARDEN_SOCK", "/host/path.sock")

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceEnvVar, Value: "TEST_WARDEN_SOCK"}},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "SOCK_PATH", Value: "/container/path.sock"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if cred.Injections[0].Value != "/container/path.sock" {
		t.Errorf("expected static override, got %q", cred.Injections[0].Value)
	}
}

func TestResolve_UnresolvedCredentialSkipped(t *testing.T) {
	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Missing",
				Sources: []Source{{Type: SourceEnvVar, Value: "DEFINITELY_NOT_SET_WARDEN_TEST"}},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "NOPE"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if cred.Resolved {
		t.Fatal("expected credential to be unresolved")
	}
	if len(cred.Injections) != 0 {
		t.Errorf("expected no injections, got %d", len(cred.Injections))
	}
}

func TestResolve_TransformStripLines(t *testing.T) {
	dir := t.TempDir()
	sshConfig := filepath.Join(dir, "config")
	content := "Host *\n  IdentitiesOnly yes\n  ForwardAgent yes\n"
	if err := os.WriteFile(sshConfig, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	// For file sources with transforms, the transform operates on the
	// file content. But our current implementation returns the path, not
	// the content. Transforms on file sources are handled by the
	// entrypoint, not the resolution engine. The strip_lines transform
	// is tested here with a string value from a command source.
	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "SSH Config",
				Sources: []Source{{Type: SourceCommand, Value: "cat " + sshConfig}},
				Transform: &Transform{
					Type:   TransformStripLines,
					Params: map[string]string{"pattern": `^\s*IdentitiesOnly`},
				},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "FILTERED_CONFIG"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved")
	}
	filtered := cred.Injections[0].Value
	if filtered != "Host *\n  ForwardAgent yes" {
		t.Errorf("unexpected filtered result: %q", filtered)
	}
}

func TestResolve_MultipleInjections(t *testing.T) {
	t.Setenv("TEST_WARDEN_MULTI", "/host/agent.sock")

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceEnvVar, Value: "TEST_WARDEN_MULTI"}},
				Injections: []Injection{
					{Type: InjectionMountSocket, Key: "/run/agent.sock", ReadOnly: true},
					{Type: InjectionEnvVar, Key: "AGENT_SOCK", Value: "/run/agent.sock"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if len(cred.Injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(cred.Injections))
	}
	if cred.Injections[0].Type != InjectionMountSocket {
		t.Errorf("expected mount_socket, got %s", cred.Injections[0].Type)
	}
	if cred.Injections[1].Value != "/run/agent.sock" {
		t.Errorf("expected static value override, got %s", cred.Injections[1].Value)
	}
}

func TestExpandHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}

	tests := []struct {
		input    string
		expected string
	}{
		{"~/foo", home + "/foo"},
		{"/absolute/path", "/absolute/path"},
		{"relative/path", "relative/path"},
		{"~notahome", "~notahome"},
	}

	for _, tt := range tests {
		got := expandHome(tt.input)
		if got != tt.expected {
			t.Errorf("expandHome(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

// mockEnvResolver is a test resolver that returns values from a fixed map,
// proving that the EnvResolver parameter is actually used by Resolve/Detect.
type mockEnvResolver struct {
	vars map[string]string
}

func (m *mockEnvResolver) LookupEnv(key string) (string, bool) {
	v, ok := m.vars[key]
	return v, ok
}

func (m *mockEnvResolver) ExpandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		return m.vars[key]
	})
}

func (m *mockEnvResolver) Environ() []string {
	var env []string
	for k, v := range m.vars {
		env = append(env, k+"="+v)
	}
	return env
}

func TestResolve_WithCustomEnvResolver(t *testing.T) {
	// This env var is NOT set in the process — only in the mock resolver.
	resolver := &mockEnvResolver{
		vars: map[string]string{
			"MOCK_ONLY_TOKEN": "secret-from-shell",
		},
	}

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Token",
				Sources: []Source{{Type: SourceEnvVar, Value: "MOCK_ONLY_TOKEN"}},
				Injections: []Injection{
					{Type: InjectionEnvVar, Key: "CONTAINER_TOKEN"},
				},
			},
		},
	}

	result, err := Resolve(item, resolver)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved via custom resolver")
	}
	if cred.Injections[0].Value != "secret-from-shell" {
		t.Errorf("expected 'secret-from-shell', got %q", cred.Injections[0].Value)
	}
}

func TestDetect_SocketSource_LiveSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "agent.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() //nolint:errcheck

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: sockPath}},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected live socket to be detected as available")
	}
}

// createStaleSocket creates a real Unix socket file on disk with no active
// listener. Uses SetUnlinkOnClose(false) to retain the file after the
// listener closes on platforms that would otherwise remove it.
func createStaleSocket(t *testing.T, path string) {
	t.Helper()
	ln, err := net.Listen("unix", path)
	if err != nil {
		t.Fatal(err)
	}
	ln.(*net.UnixListener).SetUnlinkOnClose(false)
	_ = ln.Close()
	fi, statErr := os.Stat(path)
	if statErr != nil {
		t.Fatalf("stale socket file should still exist: %v", statErr)
	}
	if fi.Mode().Type() != os.ModeSocket {
		t.Fatalf("expected socket type, got %v", fi.Mode().Type())
	}
}

func TestDetect_SocketSource_StaleSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "stale.sock")
	createStaleSocket(t, sockPath)

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: sockPath}},
			},
		},
	}

	result := Detect(item, nil)
	if result.Available {
		t.Fatal("expected stale socket to be detected as unavailable")
	}
}

func TestResolve_SocketSource_LiveSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "agent.sock")

	ln, err := net.Listen("unix", sockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() //nolint:errcheck

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: sockPath}},
				Injections: []Injection{
					{Type: InjectionMountSocket, Key: "/run/agent.sock"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if !cred.Resolved {
		t.Fatal("expected credential to be resolved for live socket")
	}
	if cred.Injections[0].Value != sockPath {
		t.Errorf("expected %s, got %s", sockPath, cred.Injections[0].Value)
	}
}

func TestResolve_SocketSource_StaleSocket(t *testing.T) {
	dir := t.TempDir()
	sockPath := filepath.Join(dir, "stale.sock")
	createStaleSocket(t, sockPath)

	item := Item{
		ID:     "test",
		Label:  "Test",
		Method: MethodTransport,
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: sockPath}},
				Injections: []Injection{
					{Type: InjectionMountSocket, Key: "/run/agent.sock"},
				},
			},
		},
	}

	result, err := Resolve(item, nil)
	if err != nil {
		t.Fatal(err)
	}

	cred := result.Credentials[0]
	if cred.Resolved {
		t.Fatal("expected stale socket credential to be unresolved")
	}
}

func TestDetect_SocketSource_SymlinkToLiveSocket(t *testing.T) {
	dir := t.TempDir()
	realSockPath := filepath.Join(dir, "real.sock")
	symlinkPath := filepath.Join(dir, "link.sock")

	ln, err := net.Listen("unix", realSockPath)
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close() //nolint:errcheck

	if err := os.Symlink(realSockPath, symlinkPath); err != nil {
		t.Fatal(err)
	}

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: symlinkPath}},
			},
		},
	}

	result := Detect(item, nil)
	if !result.Available {
		t.Fatal("expected symlink to live socket to be detected as available")
	}
}

func TestDetect_SocketSource_SymlinkToNonexistent(t *testing.T) {
	dir := t.TempDir()
	symlinkPath := filepath.Join(dir, "broken.sock")

	if err := os.Symlink("/nonexistent/socket.sock", symlinkPath); err != nil {
		t.Fatal(err)
	}

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Agent",
				Sources: []Source{{Type: SourceSocketPath, Value: symlinkPath}},
			},
		},
	}

	result := Detect(item, nil)
	if result.Available {
		t.Fatal("expected broken symlink to be detected as unavailable")
	}
}

func TestDetect_WithCustomEnvResolver(t *testing.T) {
	resolver := &mockEnvResolver{
		vars: map[string]string{
			"MOCK_DETECT_VAR": "present",
		},
	}

	item := Item{
		ID:    "test",
		Label: "Test",
		Credentials: []Credential{
			{
				Label:   "Var",
				Sources: []Source{{Type: SourceEnvVar, Value: "MOCK_DETECT_VAR"}},
			},
		},
	}

	result := Detect(item, resolver)
	if !result.Available {
		t.Fatal("expected item to be available via custom resolver")
	}
	if !result.Credentials[0].Available {
		t.Fatal("expected credential to be available via custom resolver")
	}
}

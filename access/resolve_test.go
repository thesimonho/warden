package access

import (
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

	result := Detect(item)
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

	result := Detect(item)
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

	result := Detect(item)
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

	result := Detect(item)
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

	result := Detect(item)
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

	result := Detect(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

	result, err := Resolve(item)
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

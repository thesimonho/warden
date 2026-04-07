package server

import "testing"

func TestParseProxySubdomain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		subdomain string
		wantPID   string
		wantAT    string
		wantPort  int
		wantOK    bool
	}{
		{
			name:      "claude-code with port",
			subdomain: "04f400635297-claude-code-5173",
			wantPID:   "04f400635297",
			wantAT:    "claude-code",
			wantPort:  5173,
			wantOK:    true,
		},
		{
			name:      "codex with port",
			subdomain: "abcdef123456-codex-3000",
			wantPID:   "abcdef123456",
			wantAT:    "codex",
			wantPort:  3000,
			wantOK:    true,
		},
		{
			name:      "port 1 (min)",
			subdomain: "aabbccddee01-codex-1",
			wantPID:   "aabbccddee01",
			wantAT:    "codex",
			wantPort:  1,
			wantOK:    true,
		},
		{
			name:      "port 65535 (max)",
			subdomain: "aabbccddee01-codex-65535",
			wantPID:   "aabbccddee01",
			wantAT:    "codex",
			wantPort:  65535,
			wantOK:    true,
		},
		{
			name:      "empty string",
			subdomain: "",
			wantOK:    false,
		},
		{
			name:      "no hyphens",
			subdomain: "nohyphens",
			wantOK:    false,
		},
		{
			name:      "missing port",
			subdomain: "04f400635297-claude-code",
			wantOK:    false,
		},
		{
			name:      "non-numeric port",
			subdomain: "04f400635297-claude-code-abc",
			wantOK:    false,
		},
		{
			name:      "port zero",
			subdomain: "04f400635297-claude-code-0",
			wantOK:    false,
		},
		{
			name:      "port over 65535",
			subdomain: "04f400635297-claude-code-70000",
			wantOK:    false,
		},
		{
			name:      "missing project ID",
			subdomain: "-claude-code-5173",
			wantOK:    false,
		},
		{
			name:      "missing agent type",
			subdomain: "04f400635297--5173",
			wantOK:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			pid, at, port, ok := parseProxySubdomain(tt.subdomain)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tt.wantOK)
			}
			if !ok {
				return
			}
			if pid != tt.wantPID {
				t.Errorf("projectID = %q, want %q", pid, tt.wantPID)
			}
			if at != tt.wantAT {
				t.Errorf("agentType = %q, want %q", at, tt.wantAT)
			}
			if port != tt.wantPort {
				t.Errorf("port = %d, want %d", port, tt.wantPort)
			}
		})
	}
}

package server

import (
	"net/url"
	"testing"
)

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

func TestRewriteReferer(t *testing.T) {
	t.Parallel()

	target, _ := url.Parse("http://172.17.0.2:8081")

	tests := []struct {
		name    string
		referer string
		want    string
	}{
		{
			name:    "rewrites authority preserving path",
			referer: "http://abc123-claude-code-8081.localhost:8090/some/page?q=1",
			want:    "http://172.17.0.2:8081/some/page?q=1",
		},
		{
			name:    "rewrites authority with no path",
			referer: "http://abc123-claude-code-8081.localhost:8090",
			want:    "http://172.17.0.2:8081",
		},
		{
			name:    "empty referer unchanged",
			referer: "",
			want:    "http://172.17.0.2:8081",
		},
		{
			name:    "preserves fragment",
			referer: "http://abc123-claude-code-8081.localhost:8090/page#section",
			want:    "http://172.17.0.2:8081/page#section",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteReferer(tt.referer, target)
			if got != tt.want {
				t.Errorf("rewriteReferer() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRewriteLocation(t *testing.T) {
	t.Parallel()

	target, _ := url.Parse("http://172.17.0.2:8081")
	proxyOrigin := "http://abc123-claude-code-8081.localhost:8090"

	tests := []struct {
		name string
		loc  string
		want string
	}{
		{
			name: "rewrites target origin to proxy origin",
			loc:  "http://172.17.0.2:8081/login?next=/dashboard",
			want: "http://abc123-claude-code-8081.localhost:8090/login?next=/dashboard",
		},
		{
			name: "leaves relative path unchanged",
			loc:  "/login",
			want: "/login",
		},
		{
			name: "leaves unrelated origin unchanged",
			loc:  "http://example.com/other",
			want: "http://example.com/other",
		},
		{
			name: "rewrites root redirect",
			loc:  "http://172.17.0.2:8081/",
			want: "http://abc123-claude-code-8081.localhost:8090/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := rewriteLocation(tt.loc, target, proxyOrigin)
			if got != tt.want {
				t.Errorf("rewriteLocation() = %q, want %q", got, tt.want)
			}
		})
	}
}

//go:build !windows

package access

import "testing"

func TestGPGAgentCredential(t *testing.T) {
	cred := gpgAgentCredential()

	// Two sources: XDG runtime dir (systemd) and HOME fallback (traditional/Homebrew).
	if len(cred.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(cred.Sources))
	}
	if cred.Sources[0].Type != SourceSocketPath {
		t.Errorf("expected socket source, got %s", cred.Sources[0].Type)
	}
	if cred.Sources[1].Type != SourceSocketPath {
		t.Errorf("expected socket source, got %s", cred.Sources[1].Type)
	}

	if len(cred.Injections) != 1 {
		t.Fatalf("expected 1 injection, got %d", len(cred.Injections))
	}

	mountInj := cred.Injections[0]
	if mountInj.Type != InjectionMountSocket {
		t.Errorf("expected mount_socket injection, got %s", mountInj.Type)
	}
	if mountInj.Key != ContainerGPGAgentPath {
		t.Errorf("expected container target %q, got %q", ContainerGPGAgentPath, mountInj.Key)
	}
	// No mount source override — Docker Desktop does not proxy GPG.
	if mountInj.Value != "" {
		t.Errorf("expected no mount source override, got %q", mountInj.Value)
	}
}

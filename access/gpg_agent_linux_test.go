//go:build linux

package access

import "testing"

func TestGPGAgentCredential_Linux(t *testing.T) {
	cred := gpgAgentCredential()

	if len(cred.Sources) != 2 {
		t.Fatalf("expected 2 sources (XDG + HOME fallback), got %d", len(cred.Sources))
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
	if mountInj.Key != containerGPGAgentPath {
		t.Errorf("expected container target %q, got %q", containerGPGAgentPath, mountInj.Key)
	}
	// On Linux, mount source is not overridden — uses the resolved
	// socket path directly.
	if mountInj.Value != "" {
		t.Errorf("expected no mount source override on Linux, got %q", mountInj.Value)
	}
}

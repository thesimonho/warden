//go:build darwin

package access

import "testing"

func TestGPGAgentCredential_Darwin(t *testing.T) {
	cred := gpgAgentCredential()

	if len(cred.Sources) != 1 || cred.Sources[0].Type != SourceSocketPath {
		t.Fatal("expected single socket source for GPG agent detection")
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
	// On macOS, no Docker Desktop proxy exists for GPG (unlike SSH),
	// so no mount source override is applied.
	if mountInj.Value != "" {
		t.Errorf("expected no mount source override on macOS, got %q", mountInj.Value)
	}
}

//go:build windows

package access

import "testing"

func TestGPGAgentCredential_Windows(t *testing.T) {
	cred := gpgAgentCredential()

	// On Windows, the GPG agent credential has no sources — it will
	// never be detected, so the agent mount is cleanly skipped.
	if len(cred.Sources) != 0 {
		t.Errorf("expected no sources on Windows, got %d", len(cred.Sources))
	}
	if len(cred.Injections) != 0 {
		t.Errorf("expected no injections on Windows, got %d", len(cred.Injections))
	}
}

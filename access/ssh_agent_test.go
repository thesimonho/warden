//go:build !windows

package access

import "testing"

func TestSSHAgentCredential(t *testing.T) {
	cred := sshAgentCredential()

	if len(cred.Sources) != 1 || cred.Sources[0].Type != SourceSocketPath {
		t.Fatal("expected single socket source for SSH agent detection")
	}
	if cred.Sources[0].Value != "$SSH_AUTH_SOCK" {
		t.Errorf("expected $SSH_AUTH_SOCK source, got %q", cred.Sources[0].Value)
	}

	if len(cred.Injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(cred.Injections))
	}

	mountInj := cred.Injections[0]
	if mountInj.Type != InjectionMountSocket {
		t.Errorf("expected mount_socket injection, got %s", mountInj.Type)
	}
	if mountInj.Key != ContainerSSHAgentPath {
		t.Errorf("expected container target %q, got %q", ContainerSSHAgentPath, mountInj.Key)
	}
	// No mount source override — Docker Desktop proxy is applied by the
	// service layer, not baked into the credential definition.
	if mountInj.Value != "" {
		t.Errorf("expected no mount source override, got %q", mountInj.Value)
	}

	envInj := cred.Injections[1]
	if envInj.Type != InjectionEnvVar {
		t.Errorf("expected env injection, got %s", envInj.Type)
	}
	if envInj.Value != ContainerSSHAgentPath {
		t.Errorf("expected SSH_AUTH_SOCK=%q, got %q", ContainerSSHAgentPath, envInj.Value)
	}
}

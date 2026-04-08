//go:build darwin

package access

import "testing"

func TestSSHAgentCredential_Darwin(t *testing.T) {
	cred := sshAgentCredential()

	if len(cred.Sources) != 1 || cred.Sources[0].Type != SourceSocketPath {
		t.Fatal("expected socket source for SSH agent detection")
	}

	if len(cred.Injections) != 2 {
		t.Fatalf("expected 2 injections, got %d", len(cred.Injections))
	}

	mountInj := cred.Injections[0]
	if mountInj.Type != InjectionMountSocket {
		t.Errorf("expected mount_socket injection, got %s", mountInj.Type)
	}
	if mountInj.Value != dockerDesktopSSHAgentPath {
		t.Errorf("expected mount source override to Docker Desktop proxy %q, got %q",
			dockerDesktopSSHAgentPath, mountInj.Value)
	}
	if mountInj.Key != containerSSHAgentPath {
		t.Errorf("expected container target %q, got %q", containerSSHAgentPath, mountInj.Key)
	}

	envInj := cred.Injections[1]
	if envInj.Type != InjectionEnvVar {
		t.Errorf("expected env injection, got %s", envInj.Type)
	}
	if envInj.Value != containerSSHAgentPath {
		t.Errorf("expected SSH_AUTH_SOCK=%q, got %q", containerSSHAgentPath, envInj.Value)
	}
}

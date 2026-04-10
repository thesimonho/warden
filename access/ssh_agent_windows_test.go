//go:build windows

package access

import "testing"

func TestSSHAgentCredential_Windows(t *testing.T) {
	cred := sshAgentCredential()

	if len(cred.Sources) != 1 || cred.Sources[0].Type != SourceNamedPipe {
		t.Fatal("expected named pipe source for SSH agent detection on Windows")
	}
	if cred.Sources[0].Value != sshAgentPipePath {
		t.Errorf("expected pipe path %q, got %q", sshAgentPipePath, cred.Sources[0].Value)
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

	envInj := cred.Injections[1]
	if envInj.Type != InjectionEnvVar {
		t.Errorf("expected env injection, got %s", envInj.Type)
	}
	if envInj.Value != ContainerSSHAgentPath {
		t.Errorf("expected SSH_AUTH_SOCK=%q, got %q", ContainerSSHAgentPath, envInj.Value)
	}
}

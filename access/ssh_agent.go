//go:build !windows

package access

// sshAgentCredential returns the SSH Agent credential for Unix platforms.
// Detection checks $SSH_AUTH_SOCK on the host to confirm an SSH agent is
// running. The socket is mounted directly into the container.
//
// On Docker Desktop (macOS or Linux), the service layer overrides the
// mount source to use Docker Desktop's built-in SSH proxy instead,
// since host sockets cannot be bind-mounted through the VM layer.
func sshAgentCredential() Credential {
	return Credential{
		Label: "SSH Agent",
		Sources: []Source{
			{Type: SourceSocketPath, Value: "$SSH_AUTH_SOCK"},
		},
		Injections: []Injection{
			{
				Type: InjectionMountSocket,
				Key:  ContainerSSHAgentPath,
			},
			{
				Type:  InjectionEnvVar,
				Key:   "SSH_AUTH_SOCK",
				Value: ContainerSSHAgentPath,
			},
		},
	}
}

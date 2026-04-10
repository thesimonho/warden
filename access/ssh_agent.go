//go:build !windows

package access

// sshAgentCredential returns the SSH Agent credential for Unix platforms.
// Detection checks $SSH_AUTH_SOCK on the host to confirm an SSH agent is
// running. The service layer starts a TCP bridge proxy that forwards
// connections from the container (via host.docker.internal) to the host
// socket. This works identically on native Docker and Docker Desktop.
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

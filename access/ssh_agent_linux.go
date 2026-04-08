//go:build linux

package access

// sshAgentCredential returns the SSH Agent credential for Linux.
// On Linux, the host's $SSH_AUTH_SOCK socket is bind-mounted directly
// into the container. This respects the user's agent choice (system
// agent, 1Password, GPG agent, etc.).
func sshAgentCredential() Credential {
	return Credential{
		Label: "SSH Agent",
		Sources: []Source{
			{Type: SourceSocketPath, Value: "$SSH_AUTH_SOCK"},
		},
		Injections: []Injection{
			{
				Type: InjectionMountSocket,
				Key:  containerSSHAgentPath,
			},
			{
				Type:  InjectionEnvVar,
				Key:   "SSH_AUTH_SOCK",
				Value: containerSSHAgentPath,
			},
		},
	}
}

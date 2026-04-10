//go:build windows

package access

// sshAgentPipePath is the standard Windows named pipe for the OpenSSH
// SSH agent service.
const sshAgentPipePath = `\\.\pipe\openssh-ssh-agent`

// sshAgentCredential returns the SSH Agent credential for Windows.
// Detection probes the OpenSSH named pipe. The TCP socket bridge in
// the service layer dials the named pipe on the host side, while the
// container side uses socat via host.docker.internal — identical to
// Unix platforms.
func sshAgentCredential() Credential {
	return Credential{
		Label: "SSH Agent",
		Sources: []Source{
			{Type: SourceNamedPipe, Value: sshAgentPipePath},
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

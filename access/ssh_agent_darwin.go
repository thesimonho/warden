//go:build darwin

package access

// dockerDesktopSSHAgentPath is the VM-internal path where Docker Desktop
// proxies the host's SSH agent. This path exists inside Docker Desktop's
// Linux VM and is always available — it does not go through virtioFS,
// bypassing the "operation not supported" error when mounting macOS
// launchd sockets directly.
const dockerDesktopSSHAgentPath = "/run/host-services/ssh-auth.sock"

// sshAgentCredential returns the SSH Agent credential for macOS.
// Detection uses $SSH_AUTH_SOCK to confirm the host has an SSH agent,
// but the mount source is overridden to Docker Desktop's built-in proxy
// because macOS sockets cannot be bind-mounted through the VM's
// filesystem layer (virtioFS, gRPC-FUSE, or Docker VMM).
func sshAgentCredential() Credential {
	return Credential{
		Label: "SSH Agent",
		Sources: []Source{
			{Type: SourceSocketPath, Value: "$SSH_AUTH_SOCK"},
		},
		Injections: []Injection{
			{
				Type:  InjectionMountSocket,
				Key:   containerSSHAgentPath,
				Value: dockerDesktopSSHAgentPath,
			},
			{
				Type:  InjectionEnvVar,
				Key:   "SSH_AUTH_SOCK",
				Value: containerSSHAgentPath,
			},
		},
	}
}

//go:build windows

package access

// sshAgentCredential returns the SSH Agent credential for Windows.
// Windows uses named pipes (not Unix sockets) for the SSH agent, and
// Docker Desktop on Windows does not support forwarding them into
// Linux containers. The SSH access item still mounts config and
// known_hosts — SSH works if the user has passwordless keys.
// The credential is returned with no sources so it is never detected
// as available, and the agent mount is cleanly skipped.
func sshAgentCredential() Credential {
	return Credential{
		Label:   "SSH Agent",
		Sources: []Source{},
	}
}

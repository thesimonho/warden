//go:build windows

package access

// gpgAgentCredential returns the GPG Agent credential for Windows.
// Windows GPG uses Assuan pipes (not Unix sockets), and Docker
// Desktop on Windows cannot forward them into Linux containers.
// The credential is returned with no sources so it is never
// detected as available, and the socket mount is cleanly skipped.
func gpgAgentCredential() Credential {
	return Credential{
		Label:   "GPG Agent",
		Sources: []Source{},
	}
}

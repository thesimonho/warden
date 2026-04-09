//go:build darwin

package access

// gpgAgentCredential returns the GPG Agent credential for macOS.
// Detection checks for the socket at the standard Homebrew GnuPG
// location. Unlike SSH, Docker Desktop does not provide a built-in
// GPG agent proxy, so the socket is mounted directly. This works
// with some Docker Desktop configurations but may fail if the
// socket path is not accessible through the VM filesystem layer.
func gpgAgentCredential() Credential {
	return Credential{
		Label: "GPG Agent",
		Sources: []Source{
			{Type: SourceSocketPath, Value: "$HOME/.gnupg/S.gpg-agent"},
		},
		Injections: []Injection{
			{
				Type: InjectionMountSocket,
				Key:  containerGPGAgentPath,
			},
		},
	}
}

//go:build linux

package access

// gpgAgentCredential returns the GPG Agent credential for Linux.
// The host's gpg-agent socket is bind-mounted directly into the
// container at the default gpg socket path so gpg finds it
// automatically. Two source paths are tried: the systemd-managed
// location ($XDG_RUNTIME_DIR/gnupg/) and the traditional ~/.gnupg/.
func gpgAgentCredential() Credential {
	return Credential{
		Label: "GPG Agent",
		Sources: []Source{
			{Type: SourceSocketPath, Value: "$XDG_RUNTIME_DIR/gnupg/S.gpg-agent"},
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

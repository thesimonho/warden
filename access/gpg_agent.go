//go:build !windows

package access

// gpgAgentCredential returns the GPG Agent credential for Unix platforms.
// The host's gpg-agent socket is forwarded into the container via a TCP
// bridge proxy at the default gpg socket path so gpg finds it
// automatically. Two source paths are tried: the systemd-managed
// location ($XDG_RUNTIME_DIR/gnupg/) and the traditional ~/.gnupg/
// (used on macOS/Homebrew and legacy Linux setups).
//
// The bridge approach works identically on native Docker and Docker
// Desktop — no special handling or VM proxies are needed.
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
				Key:  ContainerGPGAgentPath,
			},
		},
	}
}

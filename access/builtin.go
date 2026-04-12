package access

import "github.com/thesimonho/warden/constants"

// Built-in access item IDs. These are stable identifiers stored in the
// database and referenced by frontends.
const (
	BuiltInIDGit = "git"
	BuiltInIDSSH = "ssh"
	BuiltInIDGPG = "gpg"
)

// ContainerGitConfigHostPath is the container path where the host's
// gitconfig is mounted. Used by the entrypoint to set up
// `git config --global include.path` and by the TransformGitInclude
// transform to produce content-bearing injections.
const ContainerGitConfigHostPath = constants.ContainerHomeDir + "/.gitconfig.host"

// ContainerGitIncludeDir is the directory inside the container where
// git include files from the host are mounted. The TransformGitInclude
// transform rewrites include paths to point here.
const ContainerGitIncludeDir = constants.ContainerHomeDir + "/.gitconfig.d"

// ContainerSSHAgentPath is the fixed path where the SSH agent socket
// appears inside the container. The entrypoint's socat process creates
// this socket and forwards connections to the host via the TCP bridge.
// Placed under the warden user's home directory so the entrypoint can
// create it without root permissions.
const ContainerSSHAgentPath = constants.ContainerHomeDir + "/.ssh/agent.sock"

// ContainerGPGAgentPath is the fixed path where the GPG agent socket
// appears inside the container. Placed at the default gpg socket
// location (~/.gnupg/S.gpg-agent) so gpg finds it automatically
// without needing env var overrides or extra configuration.
const ContainerGPGAgentPath = constants.ContainerHomeDir + "/.gnupg/S.gpg-agent"

// ContainerGPGPubringPath is where the host's public keyring is
// mounted inside the container. GPG needs the public keyring to
// know which keys exist — the agent socket alone only handles
// private key operations (signing/decryption).
const ContainerGPGPubringPath = constants.ContainerHomeDir + "/.gnupg/pubring.kbx"

// ContainerGPGTrustDBPath is where the host's trust database is
// mounted. Without it, GPG shows "[unknown]" trust level for all
// keys, which can cause signing failures with strict trust settings.
const ContainerGPGTrustDBPath = constants.ContainerHomeDir + "/.gnupg/trustdb.gpg"

// BuiltInGit returns the built-in Git access item. It mounts the host's
// .gitconfig (read-only) so git commands inside the container use the
// host user's identity and settings.
func BuiltInGit() Item {
	return Item{
		ID:          BuiltInIDGit,
		Label:       "Git",
		Description: "Mounts host .gitconfig read-only so git commands use your identity and settings.",
		Method:      MethodTransport,
		BuiltIn:     true,
		Credentials: []Credential{
			{
				Label: "Git Config",
				Sources: []Source{
					{Type: SourceFilePath, Value: "~/.gitconfig"},
					{Type: SourceFilePath, Value: "~/.config/git/config"},
				},
				Transform: &Transform{Type: TransformGitInclude},
				Injections: []Injection{
					{
						Type:     InjectionMountFile,
						Key:      ContainerGitConfigHostPath,
						ReadOnly: true,
					},
				},
			},
		},
	}
}

// BuiltInSSH returns the built-in SSH access item. It mounts the host's
// SSH config (filtered to strip IdentitiesOnly), known_hosts, and
// optionally forwards the SSH agent socket.
func BuiltInSSH() Item {
	return Item{
		ID:          BuiltInIDSSH,
		Label:       "SSH",
		Description: "Mounts SSH config and known_hosts. Forwards the ssh-agent socket so SSH works without copying keys.",
		Method:      MethodTransport,
		BuiltIn:     true,
		Credentials: []Credential{
			{
				Label: "SSH Config",
				Sources: []Source{
					{Type: SourceFilePath, Value: "~/.ssh/config"},
				},
				Transform: &Transform{
					Type:   TransformStripLines,
					Params: map[string]string{"pattern": `^\s*IdentitiesOnly`},
				},
				Injections: []Injection{
					{
						Type:     InjectionMountFile,
						Key:      constants.ContainerHomeDir + "/.ssh/config.host",
						ReadOnly: true,
					},
				},
			},
			{
				Label: "SSH Known Hosts",
				Sources: []Source{
					{Type: SourceFilePath, Value: "~/.ssh/known_hosts"},
				},
				Injections: []Injection{
					{
						Type: InjectionMountFile,
						Key:  constants.ContainerHomeDir + "/.ssh/known_hosts",
					},
				},
			},
			sshAgentCredential(),
		},
	}
}

// BuiltInGPG returns the built-in GPG access item. It forwards the
// host's gpg-agent socket and mounts the public keyring so git commit
// signing (-S) works inside the container without copying private keys.
// The public keyring is needed because GPG must know which keys exist
// before it can ask the agent to perform signing operations.
func BuiltInGPG() Item {
	return Item{
		ID:          BuiltInIDGPG,
		Label:       "GPG",
		Description: "Forwards the gpg-agent socket and mounts the public keyring so commit signing works without copying private keys.",
		Method:      MethodTransport,
		BuiltIn:     true,
		Credentials: []Credential{
			gpgAgentCredential(),
			{
				Label: "GPG Public Keyring",
				Sources: []Source{
					{Type: SourceFilePath, Value: "~/.gnupg/pubring.kbx"},
				},
				Injections: []Injection{
					{
						Type:     InjectionMountFile,
						Key:      ContainerGPGPubringPath,
						ReadOnly: true,
					},
				},
			},
			{
				Label: "GPG Trust Database",
				Sources: []Source{
					{Type: SourceFilePath, Value: "~/.gnupg/trustdb.gpg"},
				},
				Injections: []Injection{
					{
						Type:     InjectionMountFile,
						Key:      ContainerGPGTrustDBPath,
						ReadOnly: true,
					},
				},
			},
		},
	}
}

// BuiltInItems returns all built-in access items.
func BuiltInItems() []Item {
	return []Item{
		BuiltInGit(),
		BuiltInSSH(),
		BuiltInGPG(),
	}
}

// BuiltInItemByID returns a built-in access item by ID, or nil if not found.
func BuiltInItemByID(id string) *Item {
	for _, item := range BuiltInItems() {
		if item.ID == id {
			return &item
		}
	}
	return nil
}

// IsBuiltInID reports whether the given ID belongs to a built-in access item.
func IsBuiltInID(id string) bool {
	return BuiltInItemByID(id) != nil
}

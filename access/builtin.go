package access

import "github.com/thesimonho/warden/constants"

// Built-in access item IDs. These are stable identifiers stored in the
// database and referenced by frontends.
const (
	BuiltInIDGit = "git"
	BuiltInIDSSH = "ssh"
)

// containerSSHAgentPath is the fixed path where the host's SSH agent
// socket is mounted inside the container.
const containerSSHAgentPath = "/run/ssh-agent.sock"

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
						Key:      constants.ContainerHomeDir + "/.gitconfig.host",
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
			{
				Label: "SSH Agent",
				Sources: []Source{
					{Type: SourceSocketPath, Value: "$SSH_AUTH_SOCK"},
				},
				Injections: []Injection{
					{
						Type:     InjectionMountSocket,
						Key:      containerSSHAgentPath,
						ReadOnly: true,
					},
					{
						Type:  InjectionEnvVar,
						Key:   "SSH_AUTH_SOCK",
						Value: containerSSHAgentPath,
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
	return id == BuiltInIDGit || id == BuiltInIDSSH
}

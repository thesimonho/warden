// Package access defines the credential passthrough model for Warden
// containers. An [Item] groups one or more [Credential] entries that
// share host access with a container. Each credential declares how to
// detect and read a value on the host ([Source]), an optional
// transformation ([Transform]), and how to deliver the result into the
// container ([Injection]).
//
// Resolution is stateless — Warden never stores credential values, only
// the recipes that describe how to obtain and inject them.
package access

import "time"

// SourceType describes where a credential value lives on the host.
type SourceType string

const (
	// SourceEnvVar reads a value from a host environment variable.
	SourceEnvVar SourceType = "env"
	// SourceFilePath reads a file from the host filesystem.
	SourceFilePath SourceType = "file"
	// SourceSocketPath references a Unix domain socket on the host.
	SourceSocketPath SourceType = "socket"
	// SourceCommand runs a host command and captures stdout.
	SourceCommand SourceType = "command"
	// SourceNamedPipe references a Windows named pipe on the host
	// (e.g. \\.\pipe\openssh-ssh-agent). Detection dials the pipe
	// to verify it has a listener.
	SourceNamedPipe SourceType = "named_pipe"
)

// ProbeTimeout is the maximum time to wait when verifying a host socket
// or named pipe has a live listener. Kept short since these are local.
const ProbeTimeout = 500 * time.Millisecond

// InjectionType describes how a resolved credential is delivered into
// the container.
type InjectionType string

const (
	// InjectionEnvVar sets an environment variable inside the container.
	InjectionEnvVar InjectionType = "env"
	// InjectionMountFile bind-mounts a host file into the container.
	InjectionMountFile InjectionType = "mount_file"
	// InjectionMountSocket signals that a Unix domain socket should be
	// forwarded into the container. The service layer bridges the host
	// socket via a TCP proxy; socat in the container recreates it.
	InjectionMountSocket InjectionType = "mount_socket"
)

// Method identifies the strategy used to deliver credentials into a
// container. Only [MethodTransport] is implemented; the interface exists
// so a proxy-based method can be added later without changing callers.
type Method string

const (
	// MethodTransport extracts a credential on the host and injects it
	// directly into the container (env var, bind mount, or socket bridge).
	MethodTransport Method = "transport"
)

// TransformType identifies a built-in transformation applied between
// source resolution and container injection. Transforms are internal —
// they are not exposed in the user-facing UI.
type TransformType string

const (
	// TransformStripLines removes lines matching a case-insensitive
	// pattern. Params: "pattern" (regex).
	TransformStripLines TransformType = "strip_lines"
	// TransformGitInclude writes an include directive rather than
	// mounting over the container's .gitconfig.
	TransformGitInclude TransformType = "git_include"
)

// Source describes how to detect and read a credential value on the
// host. Detection is implicit: an env var must be set, a file or socket
// must exist, and a command must exit 0.
type Source struct {
	// Type is the kind of host source.
	Type SourceType `json:"type"`
	// Value is the env var name, file path, socket path, or command string.
	Value string `json:"value"`
}

// Transform describes an optional processing step between source
// resolution and injection. Only used by built-in access items.
type Transform struct {
	// Type identifies the transformation.
	Type TransformType `json:"type"`
	// Params holds type-specific configuration (e.g. "pattern" for strip_lines).
	Params map[string]string `json:"params,omitempty"`
}

// Injection describes how a resolved credential is delivered into the
// container.
type Injection struct {
	// Type is the kind of container injection.
	Type InjectionType `json:"type"`
	// Key is the env var name or container path for the injection target.
	Key string `json:"key"`
	// Value is a static override for the resolved value. When set, this
	// is used instead of the source-resolved value. Useful when the
	// injection needs a fixed container-side path (e.g. SSH_AUTH_SOCK
	// env var pointing to the container socket path).
	Value string `json:"value,omitempty"`
	// ReadOnly applies to mount injections — when true the mount is read-only.
	ReadOnly bool `json:"readOnly,omitempty"`
}

// Credential is the atomic unit of the access system. It pairs one or
// more host [Source] entries (tried in order, first detected wins) with
// an optional [Transform] and one or more container [Injection] targets.
type Credential struct {
	// Label is a human-readable name for this credential (e.g. "SSH Agent Socket").
	Label string `json:"label"`
	// Sources are tried in order; the first detected value is used.
	Sources []Source `json:"sources"`
	// Transform is an optional processing step applied to the resolved value.
	Transform *Transform `json:"transform,omitempty"`
	// Injections are the container-side delivery targets.
	Injections []Injection `json:"injections"`
}

// Item is a named group of credentials that share host access with
// containers. Items can be built-in (shipped with Warden, not deletable)
// or user-created.
type Item struct {
	// ID is a stable identifier. Built-in items use well-known IDs
	// (e.g. "git", "ssh"); user items get generated UUIDs.
	ID string `json:"id"`
	// Label is the human-readable display name (e.g. "Git Config").
	Label string `json:"label"`
	// Description explains what this access item provides.
	Description string `json:"description"`
	// Method is the delivery strategy (only "transport" for now).
	Method Method `json:"method"`
	// Credentials are the individual credential entries in this group.
	Credentials []Credential `json:"credentials"`
	// BuiltIn is true for items that ship with Warden.
	BuiltIn bool `json:"builtIn"`
}

// --- Resolution output types ---

// ResolvedInjection is a single resolved delivery into the container.
type ResolvedInjection struct {
	// Type is the injection kind (env, mount_file, mount_socket).
	Type InjectionType `json:"type"`
	// Key is the env var name or container path.
	Key string `json:"key"`
	// Value is the resolved content (env var value, host file path,
	// or host socket path).
	Value string `json:"value"`
	// ReadOnly applies to mount injections.
	ReadOnly bool `json:"readOnly,omitempty"`
}

// ResolvedCredential holds the resolution output for a single credential.
type ResolvedCredential struct {
	// Label is the credential's human-readable name.
	Label string `json:"label"`
	// Resolved is true when a source was detected and all injections
	// were produced.
	Resolved bool `json:"resolved"`
	// SourceMatched describes which source was matched (empty when unresolved).
	SourceMatched string `json:"sourceMatched,omitempty"`
	// Injections are the resolved container-side deliveries.
	Injections []ResolvedInjection `json:"injections,omitempty"`
	// Error is set when resolution failed (distinct from "not detected").
	Error string `json:"error,omitempty"`
}

// ResolvedItem holds the full resolution output for an access item.
type ResolvedItem struct {
	// ID is the access item identifier.
	ID string `json:"id"`
	// Label is the access item display name.
	Label string `json:"label"`
	// Credentials contains per-credential resolution results.
	Credentials []ResolvedCredential `json:"credentials"`
}

// --- Detection output types ---

// CredentialStatus reports whether a single credential's sources are
// available on the host, without reading their values.
type CredentialStatus struct {
	// Label is the credential's human-readable name.
	Label string `json:"label"`
	// Available is true when at least one source was detected.
	Available bool `json:"available"`
	// SourceMatched describes which source was detected (empty when unavailable).
	SourceMatched string `json:"sourceMatched,omitempty"`
}

// DetectionResult reports which credentials within an access item are
// available on the current host.
type DetectionResult struct {
	// ID is the access item identifier.
	ID string `json:"id"`
	// Label is the access item display name.
	Label string `json:"label"`
	// Available is true when at least one credential was detected.
	Available bool `json:"available"`
	// Credentials contains per-credential detection results.
	Credentials []CredentialStatus `json:"credentials"`
}

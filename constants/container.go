// Package constants defines shared values used across Warden's packages.
// It has no imports and no logic — any package can depend on it safely.
package constants

// ContainerUser is the non-root user inside project containers. All terminal
// processes (tmux, the agent, bash) run as this user.
const ContainerUser = "warden"

// ContainerHomeDir is the home directory for [ContainerUser] inside containers.
const ContainerHomeDir = "/home/" + ContainerUser

// TmuxSessionPrefix is prepended to worktree IDs to form tmux session names.
const TmuxSessionPrefix = "warden-"

// TmuxShellSessionPrefix is prepended to worktree IDs to form the name of the
// auxiliary bash-shell tmux session that backs the Terminal tab in the UI.
// The shell session is independent from the agent tmux session — it runs a
// plain bash at the worktree's working directory and has no agent lifecycle
// or cost-tracking semantics.
const TmuxShellSessionPrefix = "warden-shell-"

// TmuxSessionName returns the tmux session name for a worktree.
func TmuxSessionName(worktreeID string) string {
	return TmuxSessionPrefix + worktreeID
}

// TmuxShellSessionName returns the tmux session name for a worktree's
// auxiliary bash-shell session (the backing session for the Terminal tab in
// the webapp and the shell attach action in the TUI). It is distinct from
// [TmuxSessionName] so the agent and shell can coexist in the same container.
func TmuxShellSessionName(worktreeID string) string {
	return TmuxShellSessionPrefix + worktreeID
}

// CreateShellScript is the in-container path to the auxiliary bash-shell
// tmux session bootstrap script, installed by install-warden.sh. It is
// exported (unlike the agent/disconnect script paths, which live privately
// in engine/worktrees.go) because both the terminal proxy and the TUI
// adapter invoke it from outside the engine package.
const CreateShellScript = "/usr/local/bin/create-shell.sh"

// LabelManaged marks a container as managed by Warden. Used by the Docker
// events watcher to filter start events and by cleanup routines to identify
// Warden-owned containers.
const LabelManaged = "app.warden.managed"

// LabelEphemeral marks a container as a short-lived helper (CLI precache,
// firewall iptables, network isolation). These are cleaned up on server
// startup in case a previous run crashed before the defer-based removal ran.
const LabelEphemeral = "app.warden.ephemeral"

// LabelMode distinguishes containers created by development builds ("dev")
// from release builds ("release"). Useful for pruning dev containers without
// affecting production ones. The value is derived from version.Version at
// container creation time.
const LabelMode = "app.warden.mode"

// ModeValue returns "dev" for development builds and "release" for
// versioned release builds. The input should be version.Version.
func ModeValue(v string) string {
	if v == "dev" {
		return "dev"
	}
	return "release"
}

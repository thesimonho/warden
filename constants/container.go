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

// TmuxSessionName returns the tmux session name for a worktree.
func TmuxSessionName(worktreeID string) string {
	return TmuxSessionPrefix + worktreeID
}

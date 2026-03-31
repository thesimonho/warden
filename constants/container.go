// Package constants defines shared values used across Warden's packages.
// It has no imports and no logic — any package can depend on it safely.
package constants

// ContainerUser is the non-root user inside project containers. All terminal
// processes (abduco, the agent, bash) run as this user.
const ContainerUser = "warden"

// ContainerHomeDir is the home directory for [ContainerUser] inside containers.
const ContainerHomeDir = "/home/" + ContainerUser

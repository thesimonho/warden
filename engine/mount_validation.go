package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/thesimonho/warden/api"
)

// StaleMountsError is returned by RestartProject when the container's bind
// mounts no longer match what a fresh symlink resolution would produce.
// This typically happens when a dotfile manager changes symlink targets
// after the container was created.
//
// Starting a container with stale mounts causes Claude Code to see
// unreadable or outdated config files. If Claude Code cannot read its
// settings, it performs a fresh write that overwrites the host's settings
// through the read-write parent directory mount — destroying user hooks,
// permissions, and preferences.
//
// The container must be recreated to pick up the current symlink targets.
type StaleMountsError struct {
	// StalePaths lists the container-side mount paths whose host-side
	// sources have diverged from a fresh resolution.
	StalePaths []string
}

// Error implements the error interface.
func (e *StaleMountsError) Error() string {
	return fmt.Sprintf(
		"bind mounts are stale (symlink targets changed since container creation): %s — recreate the container to refresh mounts",
		strings.Join(e.StalePaths, ", "),
	)
}

// IsStaleMountsError reports whether err is a StaleMountsError.
func IsStaleMountsError(err error) bool {
	var target *StaleMountsError
	return errors.As(err, &target)
}

// DetectStaleMounts re-resolves the original (pre-resolution) mount specs
// and compares the result with the container's current resolved mounts.
// Returns a list of container paths where the fresh resolution differs from
// what the container currently has.
//
// Only mount paths produced by the fresh resolution are checked. Extra
// mounts in the container that aren't derived from the original specs
// (e.g. socket mounts, cache volumes) are ignored — they aren't tracked
// in original_mounts and shouldn't trigger stale mount detection.
func DetectStaleMounts(original, current []api.Mount) []string {
	fresh, err := resolveSymlinksForMounts(original)
	if err != nil {
		return nil
	}

	// Index both sets by container path.
	freshMap := mountHostIndex(fresh)
	currentMap := mountHostIndex(current)

	var stale []string

	// Check each freshly-resolved mount against what the container has.
	// Catches: changed symlink targets, new symlinks, removed symlinks.
	for containerPath, freshHost := range freshMap {
		currentHost, exists := currentMap[containerPath]
		if !exists || freshHost != currentHost {
			stale = append(stale, containerPath)
		}
	}

	sort.Strings(stale)
	return stale
}

// mountHostIndex builds a map of containerPath → hostPath for quick lookup.
func mountHostIndex(mounts []api.Mount) map[string]string {
	m := make(map[string]string, len(mounts))
	for _, mount := range mounts {
		m[mount.ContainerPath] = mount.HostPath
	}
	return m
}

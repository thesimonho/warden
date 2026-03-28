package engine

import (
	"errors"
	"fmt"
	"sort"
	"strings"
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
// Returns a list of container paths where the current mount differs from
// what a fresh resolution would produce.
//
// This detects ANY divergence — deleted targets, changed symlink targets,
// new symlinks, or removed symlinks — not just missing files.
func DetectStaleMounts(original, current []Mount) []string {
	fresh, err := resolveSymlinksForMounts(original)
	if err != nil {
		return nil
	}

	// Index both sets by container path.
	freshMap := mountHostIndex(fresh)
	currentMap := mountHostIndex(current)

	var stale []string

	// Current mounts that no longer match a fresh resolution.
	for containerPath, currentHost := range currentMap {
		freshHost, exists := freshMap[containerPath]
		if !exists || freshHost != currentHost {
			stale = append(stale, containerPath)
		}
	}

	// Fresh mounts that don't exist in current (new symlinks appeared).
	for containerPath := range freshMap {
		if _, exists := currentMap[containerPath]; !exists {
			stale = append(stale, containerPath)
		}
	}

	sort.Strings(stale)
	return stale
}

// mountHostIndex builds a map of containerPath → hostPath for quick lookup.
func mountHostIndex(mounts []Mount) map[string]string {
	m := make(map[string]string, len(mounts))
	for _, mount := range mounts {
		m[mount.ContainerPath] = mount.HostPath
	}
	return m
}

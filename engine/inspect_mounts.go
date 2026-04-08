package engine

import (
	"strings"

	"github.com/docker/docker/api/types/container"

	"github.com/thesimonho/warden/api"
)

// InspectedMounts holds the parsed mount information from a Docker container
// inspect response. Consolidates the two-phase pattern (parse HostConfig.Binds,
// then scan info.Mounts for structured entries) into a single pass.
type InspectedMounts struct {
	// ProjectPath is the host path mounted at the workspace directory,
	// or empty for remote projects (which use Docker volumes).
	ProjectPath string
	// Mounts are the non-workspace, non-warden-internal bind mounts.
	Mounts []api.Mount
}

// parseMountsFromInspect extracts mount information from a Docker container
// inspect response. It parses both legacy Binds strings and structured Mounts
// (used for socket mounts), deduplicating by container path.
//
// workspaceDir is the container-side workspace path (e.g. /home/warden/<name>).
// Mounts targeting workspaceDir or /project are treated as the workspace mount
// and set ProjectPath rather than being included in Mounts.
//
// The event directory (/var/warden/events) and volume mounts are always excluded.
func parseMountsFromInspect(info container.InspectResponse, workspaceDir string) InspectedMounts {
	var result InspectedMounts
	seen := make(map[string]bool)

	isWorkspace := func(containerPath string) bool {
		return containerPath == workspaceDir || containerPath == "/project"
	}
	isInternal := func(containerPath string) bool {
		return containerPath == containerEventDir
	}

	// Phase 1: parse legacy Binds strings (host:container or host:container:ro).
	if info.HostConfig != nil {
		for _, bind := range info.HostConfig.Binds {
			parts := strings.SplitN(bind, ":", 2)
			if len(parts) != 2 {
				continue
			}
			hostPath := parts[0]
			remainder := parts[1]
			containerPath, suffix, _ := strings.Cut(remainder, ":")
			readOnly := suffix == "ro"

			seen[containerPath] = true

			if isWorkspace(containerPath) {
				result.ProjectPath = hostPath
			} else if !isInternal(containerPath) {
				result.Mounts = append(result.Mounts, api.Mount{
					HostPath:      hostPath,
					ContainerPath: containerPath,
					ReadOnly:      readOnly,
				})
			}
		}
	}

	// Phase 2: scan structured Mounts for bind-type entries not captured
	// from Binds. This picks up socket mounts (which use the Docker
	// structured mount API) and serves as a fallback for legacy containers.
	for _, m := range info.Mounts {
		if m.Type == "volume" || seen[m.Destination] {
			continue
		}
		seen[m.Destination] = true

		if isWorkspace(m.Destination) {
			if result.ProjectPath == "" {
				result.ProjectPath = m.Source
			}
		} else if !isInternal(m.Destination) {
			result.Mounts = append(result.Mounts, api.Mount{
				HostPath:      m.Source,
				ContainerPath: m.Destination,
				ReadOnly:      !m.RW,
			})
		}
	}

	return result
}

package engine

import (
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/thesimonho/warden/api"
)

// resolveSymlinksForMounts walks each mount's host path and finds symlinks
// whose targets are outside the mounted directory tree. For each such symlink,
// an additional mount is appended that maps the resolved target to the
// corresponding container path. This ensures symlinks managed by tools like
// Nix Home Manager or GNU Stow resolve correctly inside the container.
//
// Directory symlinks are mounted as directories (not recursively walked into).
// File symlinks are mounted as files. The original mount is always preserved
// as the first entry so that Docker processes it before the overlays.
//
// Single-file mounts (host path is a file, not a directory) are resolved
// in place — if the file is a symlink, its host path is replaced with the
// resolved target. No extra mount is added.
func resolveSymlinksForMounts(mounts []api.Mount) ([]api.Mount, error) {
	result := make([]api.Mount, 0, len(mounts))

	for _, m := range mounts {
		// Socket mounts (e.g. SSH agent) are not files or directories —
		// skip symlink resolution and pass them through as-is.
		if m.IsSocket {
			result = append(result, m)
			continue
		}
		resolved, err := resolveMount(m)
		if err != nil {
			return nil, err
		}
		result = append(result, resolved...)
	}

	return result, nil
}

// resolveMount resolves symlinks for a single mount entry. Four cases:
//  1. Regular file (not a symlink): pass through unchanged.
//  2. Symlink to a file: replace HostPath with the resolved real path.
//  3. Symlink to a directory: replace HostPath with resolved path, then walk
//     for external symlinks inside the resolved directory.
//  4. Real directory: walk for external symlinks inside it.
func resolveMount(m api.Mount) ([]api.Mount, error) {
	hostInfo, err := os.Lstat(m.HostPath)
	if err != nil {
		// Mount path doesn't exist or can't be read — pass through as-is
		// and let Docker handle the error.
		return []api.Mount{m}, nil
	}

	isSymlink := hostInfo.Mode()&os.ModeSymlink != 0

	// Resolve the symlink to its real target first, then classify.
	if isSymlink {
		resolved, err := filepath.EvalSymlinks(m.HostPath)
		if err != nil {
			// Broken or circular symlink — pass through as-is.
			return []api.Mount{m}, nil
		}

		targetInfo, err := os.Stat(resolved)
		if err != nil {
			return []api.Mount{m}, nil
		}

		if !targetInfo.IsDir() {
			// Symlink to a file — replace host path with resolved target.
			m.HostPath = resolved
			return []api.Mount{m}, nil
		}

		// Symlink to a directory — resolve the root path, then fall
		// through to the directory walk below.
		m.HostPath = resolved
	}

	// Regular file (not a symlink, not a directory) — pass through.
	if !hostInfo.IsDir() && !isSymlink {
		return []api.Mount{m}, nil
	}

	// Directory mount (real or resolved from symlink): walk for external symlinks.
	extras, err := walkForExternalSymlinks(m)
	if err != nil {
		return nil, err
	}

	result := make([]api.Mount, 0, 1+len(extras))
	result = append(result, m)
	result = append(result, extras...)
	return result, nil
}

// walkForExternalSymlinks recursively walks a mounted directory on the host,
// finding symlinks whose resolved targets fall outside the mount tree. Each
// such symlink produces an extra Mount entry. Only the mount tree itself is
// walked — external symlinked directories are not descended into (WalkDir
// does not follow symlinks), so the walk is bounded to the real directory
// tree under the mount root.
func walkForExternalSymlinks(parent api.Mount) ([]api.Mount, error) {
	var extras []api.Mount
	hostRoot := parent.HostPath

	err := filepath.WalkDir(hostRoot, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			slog.Warn("skipping unreadable path during mount symlink resolution",
				"path", path, "error", walkErr)
			return nil
		}

		// Skip the root directory itself.
		if path == hostRoot {
			return nil
		}

		// Skip top-level ephemeral directories that contain host-specific
		// runtime artifacts (e.g. Codex's tmp/path/ has hundreds of nix
		// store symlinks to sandbox binaries that don't exist in containers).
		if d.IsDir() {
			rel, relErr := filepath.Rel(hostRoot, path)
			if relErr == nil && !strings.Contains(rel, string(filepath.Separator)) {
				if rel == "tmp" || rel == "log" {
					return fs.SkipDir
				}
			}
		}

		// Only interested in symlinks.
		if d.Type()&os.ModeSymlink == 0 {
			return nil
		}

		// Resolve the symlink chain.
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			// Broken or circular symlink — skip gracefully.
			return nil
		}

		// Check if the resolved target is inside the mount tree.
		if isInsideDir(resolved, hostRoot) {
			return nil
		}

		// Compute the relative path within the mount.
		rel, err := filepath.Rel(hostRoot, path)
		if err != nil {
			return nil
		}
		containerPath := filepath.Join(parent.ContainerPath, rel)

		extras = append(extras, api.Mount{
			HostPath:      resolved,
			ContainerPath: containerPath,
			ReadOnly:      parent.ReadOnly,
		})

		// WalkDir does not follow symlinks, so there is no need to
		// return fs.SkipDir — the walker won't descend into symlinked
		// directories. (Returning fs.SkipDir on a non-directory entry
		// would skip remaining siblings, which is not what we want.)
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort extras by container path for deterministic mount ordering.
	sort.Slice(extras, func(i, j int) bool {
		return extras[i].ContainerPath < extras[j].ContainerPath
	})

	return extras, nil
}

// isInsideDir reports whether path is inside (or equal to) the directory dir.
// Both paths must be absolute and clean.
func isInsideDir(path, dir string) bool {
	cleanDir := filepath.Clean(dir)
	cleanPath := filepath.Clean(path)
	return cleanPath == cleanDir || strings.HasPrefix(cleanPath, cleanDir+string(filepath.Separator))
}

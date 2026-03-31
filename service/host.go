package service

import (
	"cmp"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/runtime"
)

// containerHomeDir is a convenience alias for engine.ContainerHomeDir.
var containerHomeDir = engine.ContainerHomeDir

// preferredMount defines a well-known host path that is useful inside
// the container. Each mount is only included if the host path exists.
type preferredMount struct {
	hostRelPath   string
	containerPath string
	readOnly      bool
	agentType     string // restricts mount to a specific agent type (empty = all)
}

// userMounts are always-present mounts that aren't part of any access item.
// Each config directory is tagged to its agent type so the form only shows
// the mount relevant to the selected agent.
var userMounts = []preferredMount{
	{hostRelPath: ".claude", containerPath: containerHomeDir + "/.claude", readOnly: false, agentType: "claude-code"},
	{hostRelPath: ".codex", containerPath: containerHomeDir + "/.codex", readOnly: false, agentType: "codex"},
}

// GetDefaults returns server-resolved default values for the create
// container form, including auto-detected bind mounts.
func (s *Service) GetDefaults() DefaultsResponse {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	var mounts []DefaultMount
	var envVars []DefaultEnvVar

	if homeDir != "" {
		for _, um := range userMounts {
			hostPath := filepath.Join(homeDir, um.hostRelPath)
			// Create the directory if it doesn't exist — these mounts are
			// mandatory for JSONL parsing and agent config passthrough.
			if _, statErr := os.Stat(hostPath); statErr != nil {
				if mkErr := os.MkdirAll(hostPath, 0o700); mkErr != nil {
					slog.Warn("failed to create agent config directory", "path", hostPath, "err", mkErr)
				}
			}
			mounts = append(mounts, DefaultMount{
				HostPath:      hostPath,
				ContainerPath: um.containerPath,
				ReadOnly:      um.readOnly,
				AgentType:     um.agentType,
			})
		}
	}

	return DefaultsResponse{
		HomeDir:          homeDir,
		ContainerHomeDir: containerHomeDir,
		Mounts:           mounts,
		EnvVars:          envVars,
	}
}

// ListDirectories returns filesystem entries at the given path for the
// browser. The path must be absolute. When includeFiles is false, only
// directories are returned (default behavior). When true, both
// directories and files are returned with IsDir set accordingly.
func (s *Service) ListDirectories(path string, includeFiles bool) ([]api.DirEntry, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute: %s", path)
	}

	path = filepath.Clean(path)

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	result := make([]api.DirEntry, 0, len(entries))
	for _, entry := range entries {
		isDir := entry.IsDir()
		if !isDir && !includeFiles {
			continue
		}
		result = append(result, api.DirEntry{
			Name:  entry.Name(),
			Path:  filepath.Join(path, entry.Name()),
			IsDir: isDir,
		})
	}

	slices.SortFunc(result, func(a, b api.DirEntry) int {
		// Directories first, then files.
		if a.IsDir != b.IsDir {
			if a.IsDir {
				return -1
			}
			return 1
		}
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return result, nil
}

// RevealInFileManager opens the given directory in the host's file
// manager. Returns an error if the path does not exist or is not a
// directory.
func (s *Service) RevealInFileManager(path string) error {
	if !filepath.IsAbs(path) {
		return ErrInvalidInput
	}
	info, err := os.Stat(path)
	if err != nil {
		return ErrNotFound
	}
	if !info.IsDir() {
		return ErrInvalidInput
	}

	var cmd *exec.Cmd
	switch goruntime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "windows":
		cmd = exec.Command("explorer", path)
	default:
		cmd = exec.Command("xdg-open", path)
	}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() { _ = cmd.Wait() }()

	return nil
}

// ListRuntimes returns available container runtimes (Docker, Podman).
func (s *Service) ListRuntimes(ctx context.Context) []runtime.RuntimeInfo {
	return runtime.DetectAvailable(ctx)
}

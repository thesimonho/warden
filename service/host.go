package service

import (
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	goruntime "runtime"
	"slices"
	"strings"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/runtime"
)

// containerUser is the non-root user inside the default Warden container image.
const containerUser = "dev"

// containerHomeDir is the home directory for the container user.
var containerHomeDir = "/home/" + containerUser

// preferredMount defines a well-known host path that is useful inside
// the container. Each mount is only included if the host path exists.
type preferredMount struct {
	hostRelPath   string
	containerPath string
	readOnly      bool
}

// preferredMounts lists host paths that are auto-detected and offered
// as default bind mounts when creating a new container.
var preferredMounts = []preferredMount{
	{hostRelPath: ".claude", containerPath: containerHomeDir + "/.claude", readOnly: false},
	{hostRelPath: ".gitconfig", containerPath: containerHomeDir + "/.gitconfig.host", readOnly: true},
	{hostRelPath: ".config/git/config", containerPath: containerHomeDir + "/.gitconfig.host", readOnly: true},
	{hostRelPath: ".ssh", containerPath: containerHomeDir + "/.ssh", readOnly: true},
}

// containerSSHAgentPath is the fixed path where the host's SSH agent
// socket is mounted inside the container.
const containerSSHAgentPath = "/run/ssh-agent.sock"

// GetDefaults returns server-resolved default values for the create
// container form, including auto-detected bind mounts and env vars.
func (s *Service) GetDefaults() DefaultsResponse {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	var mounts []DefaultMount
	if homeDir != "" {
		seen := make(map[string]bool)
		for _, pm := range preferredMounts {
			if seen[pm.containerPath] {
				continue
			}
			hostPath := filepath.Join(homeDir, pm.hostRelPath)
			if _, statErr := os.Stat(hostPath); statErr == nil {
				mounts = append(mounts, DefaultMount{
					HostPath:      hostPath,
					ContainerPath: pm.containerPath,
					ReadOnly:      pm.readOnly,
				})
				seen[pm.containerPath] = true
			}
		}
	}

	var envVars []DefaultEnvVar

	// Forward the host's SSH agent socket if available. This lets
	// git push/pull via SSH work without copying private keys into
	// the container — the host's already-unlocked agent handles auth.
	if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
		if fi, statErr := os.Stat(sshAuthSock); statErr == nil && fi.Mode().Type() == os.ModeSocket {
			mounts = append(mounts, DefaultMount{
				HostPath:      sshAuthSock,
				ContainerPath: containerSSHAgentPath,
				ReadOnly:      true,
			})
			envVars = append(envVars, DefaultEnvVar{
				Key:   "SSH_AUTH_SOCK",
				Value: containerSSHAgentPath,
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

// ListDirectories returns subdirectories at the given path for the
// filesystem browser. The path must be absolute.
func (s *Service) ListDirectories(path string) ([]api.DirEntry, error) {
	if !filepath.IsAbs(path) {
		return nil, fmt.Errorf("path must be absolute: %s", path)
	}

	path = filepath.Clean(path)

	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("reading directory: %w", err)
	}

	dirs := make([]api.DirEntry, 0)
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, api.DirEntry{
			Name: entry.Name(),
			Path: filepath.Join(path, entry.Name()),
		})
	}

	slices.SortFunc(dirs, func(a, b api.DirEntry) int {
		return cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	return dirs, nil
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

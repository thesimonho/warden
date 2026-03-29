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

// containerSSHAgentPath is the fixed path where the host's SSH agent
// socket is mounted inside the container.
const containerSSHAgentPath = "/run/ssh-agent.sock"

// Preset IDs for mount passthrough toggles. Stored in the DB and
// referenced by frontends to identify which presets are enabled.
const (
	PresetIDGit = "git"
	PresetIDSSH = "ssh"
)

// gitPresetMounts are the mounts for the "git" passthrough preset.
// Only .gitconfig is included — alternatives (e.g. .config/git/config)
// map to the same container path and are tried in order.
var gitPresetMounts = []preferredMount{
	{hostRelPath: ".gitconfig", containerPath: containerHomeDir + "/.gitconfig.host", readOnly: true},
	{hostRelPath: ".config/git/config", containerPath: containerHomeDir + "/.gitconfig.host", readOnly: true},
}

// sshPresetMounts are the file-based mounts for the "ssh" preset.
// The ssh-agent socket is handled separately since it's optional.
var sshPresetMounts = []preferredMount{
	{hostRelPath: ".ssh/config", containerPath: containerHomeDir + "/.ssh/config", readOnly: true},
	{hostRelPath: ".ssh/known_hosts", containerPath: containerHomeDir + "/.ssh/known_hosts", readOnly: false},
}

// userMounts are always-present mounts that aren't part of any preset.
var userMounts = []preferredMount{
	{hostRelPath: ".claude", containerPath: containerHomeDir + "/.claude", readOnly: false},
}

// buildPreset resolves a preset's mounts from host paths and returns
// the preset with Available set based on whether any mounts resolved.
func buildPreset(homeDir string, id, label, description string, candidates []preferredMount) MountPreset {
	preset := MountPreset{
		ID:          id,
		Label:       label,
		Description: description,
	}

	seen := make(map[string]bool)
	for _, pm := range candidates {
		if seen[pm.containerPath] {
			continue
		}
		hostPath := filepath.Join(homeDir, pm.hostRelPath)
		if _, err := os.Stat(hostPath); err == nil {
			preset.Mounts = append(preset.Mounts, DefaultMount{
				HostPath:      hostPath,
				ContainerPath: pm.containerPath,
				ReadOnly:      pm.readOnly,
			})
			seen[pm.containerPath] = true
		}
	}

	preset.Available = len(preset.Mounts) > 0
	return preset
}

// GetDefaults returns server-resolved default values for the create
// container form, including auto-detected bind mounts grouped into
// presets (Git, SSH) and standalone user mounts.
func (s *Service) GetDefaults() DefaultsResponse {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = ""
	}

	var presets []MountPreset
	var mounts []DefaultMount
	var envVars []DefaultEnvVar

	if homeDir != "" {
		// Git preset: .gitconfig passthrough.
		gitPreset := buildPreset(homeDir, PresetIDGit, "Git",
			"Mounts host .gitconfig read-only so git commands use your identity and settings.",
			gitPresetMounts)
		presets = append(presets, gitPreset)

		// SSH preset: config, known_hosts, and optionally the agent socket.
		sshPreset := buildPreset(homeDir, PresetIDSSH, "SSH",
			"Mounts SSH config and known_hosts. Forwards the ssh-agent socket so git over SSH works without copying keys.",
			sshPresetMounts)

		// Add ssh-agent socket if available (optional within the SSH preset).
		if sshAuthSock := os.Getenv("SSH_AUTH_SOCK"); sshAuthSock != "" {
			if fi, statErr := os.Stat(sshAuthSock); statErr == nil && fi.Mode().Type() == os.ModeSocket {
				sshPreset.Mounts = append(sshPreset.Mounts, DefaultMount{
					HostPath:      sshAuthSock,
					ContainerPath: containerSSHAgentPath,
					ReadOnly:      true,
				})
				sshPreset.EnvVars = append(sshPreset.EnvVars, DefaultEnvVar{
					Key:   "SSH_AUTH_SOCK",
					Value: containerSSHAgentPath,
				})
				sshPreset.Available = true
			}
		}

		presets = append(presets, sshPreset)

		// User mounts: always-present, not part of any preset.
		for _, um := range userMounts {
			hostPath := filepath.Join(homeDir, um.hostRelPath)
			if _, statErr := os.Stat(hostPath); statErr == nil {
				mounts = append(mounts, DefaultMount{
					HostPath:      hostPath,
					ContainerPath: um.containerPath,
					ReadOnly:      um.readOnly,
				})
			}
		}
	}

	return DefaultsResponse{
		HomeDir:          homeDir,
		ContainerHomeDir: containerHomeDir,
		Presets:          presets,
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

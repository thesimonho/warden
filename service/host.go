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
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/runtime"
	"github.com/thesimonho/warden/runtimes"
)

// containerHomeDir is the home directory of the non-root user inside containers.
var containerHomeDir = constants.ContainerHomeDir

// preferredMount defines a well-known host path that is useful inside
// the container. Each mount is only included if the host path exists.
type preferredMount struct {
	hostRelPath   string
	containerPath string
	readOnly      bool
	agentType     string // restricts mount to a specific agent type (empty = all)
	required      bool   // mandatory for the agent to function (UI prevents removal)
}

// userMounts are always-present mounts that aren't part of any access item.
// Each config directory is tagged to its agent type so the form only shows
// the mount relevant to the selected agent.
var userMounts = []preferredMount{
	{hostRelPath: ".claude", containerPath: containerHomeDir + "/.claude", readOnly: false, agentType: "claude-code", required: true},
	{hostRelPath: ".codex", containerPath: containerHomeDir + "/.codex", readOnly: false, agentType: "codex", required: true},
}

// sharedRestrictedDomains are infrastructure domains included for all agent
// types in restricted network mode. Runtime-specific domains (npm, PyPI,
// Go proxy, etc.) are managed by the runtimes package and merged at
// container creation time based on selected runtimes.
var sharedRestrictedDomains = []string{
	"*.github.com",
	"*.githubusercontent.com",
	"archive.ubuntu.com",
	"security.ubuntu.com",
}

// agentRestrictedDomains maps agent types to their API-specific domains.
var agentRestrictedDomains = map[constants.AgentType][]string{
	constants.AgentClaudeCode: {"*.anthropic.com"},
	constants.AgentCodex:      {"*.openai.com", "*.chatgpt.com"},
}

// buildRestrictedDomains returns the default allowed domains per agent type
// for the restricted network mode. Each agent gets its API domains plus
// the shared infrastructure domains.
func buildRestrictedDomains() map[string][]string {
	result := make(map[string][]string, len(agentRestrictedDomains))
	for agentType, apiDomains := range agentRestrictedDomains {
		// Copy to avoid mutating the package-level slice via append.
		combined := make([]string, 0, len(apiDomains)+len(sharedRestrictedDomains))
		combined = append(combined, apiDomains...)
		combined = append(combined, sharedRestrictedDomains...)
		result[string(agentType)] = combined
	}
	return result
}

// GetDefaults returns server-resolved default values for the create
// container form, including auto-detected bind mounts and runtimes.
// When projectPath is non-empty, runtime detection scans that directory
// for marker files.
func (s *Service) GetDefaults(projectPath string) DefaultsResponse {
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
				Required:      um.required,
			})
		}
	}

	// Build runtime defaults with detection results.
	var detected map[string]bool
	if projectPath != "" {
		detected = runtimes.Detect(projectPath)
	}

	reg := runtimes.Registry()
	runtimeDefaults := make([]api.RuntimeDefault, len(reg))
	for i, r := range reg {
		runtimeDefaults[i] = api.RuntimeDefault{
			ID:            r.ID,
			Label:         r.Label,
			Description:   r.Description,
			AlwaysEnabled: r.AlwaysEnabled,
			Detected:      detected[r.ID],
			Domains:       r.Domains,
			EnvVars:       r.EnvVars,
		}
	}

	resp := DefaultsResponse{
		HomeDir:           homeDir,
		ContainerHomeDir:  containerHomeDir,
		Mounts:            mounts,
		EnvVars:           envVars,
		RestrictedDomains: buildRestrictedDomains(),
		Runtimes:          runtimeDefaults,
	}
	if projectPath != "" {
		resp.Template = readProjectTemplate(projectPath)
	}
	return resp
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

// ListRuntimes returns available container runtimes.
func (s *Service) ListRuntimes(ctx context.Context) []runtime.RuntimeInfo {
	return runtime.DetectAvailable(ctx)
}

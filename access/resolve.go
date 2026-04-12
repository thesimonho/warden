package access

import (
	"bytes"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// defaultEnvResolver is used when nil is passed to Resolve or Detect.
var defaultEnvResolver EnvResolver = ProcessEnvResolver{}

// cachedHomeDir caches the result of os.UserHomeDir to avoid repeated syscalls.
var (
	cachedHomeDir     string
	cachedHomeDirOnce sync.Once
)

func homeDir() string {
	cachedHomeDirOnce.Do(func() {
		cachedHomeDir, _ = os.UserHomeDir()
	})
	return cachedHomeDir
}

// Resolve resolves all credentials in the given access item, returning
// the resolved injections for each credential. Credentials whose
// sources are not detected on the host are skipped (partial resolution
// is normal). An error is returned only for hard failures like an
// invalid transform configuration.
//
// If env is nil, a default [ProcessEnvResolver] is used (backward
// compatible with direct os.LookupEnv behavior).
func Resolve(item Item, env EnvResolver) (*ResolvedItem, error) {
	if env == nil {
		env = defaultEnvResolver
	}

	result := &ResolvedItem{
		ID:    item.ID,
		Label: item.Label,
	}

	for _, cred := range item.Credentials {
		resolved, err := resolveCredential(cred, env)
		if err != nil {
			return nil, fmt.Errorf("credential %q: %w", cred.Label, err)
		}
		result.Credentials = append(result.Credentials, *resolved)
	}

	return result, nil
}

// Detect checks whether each credential's sources are available on
// the host without reading their values. This is a lightweight
// availability check for the UI.
//
// If env is nil, a default [ProcessEnvResolver] is used.
func Detect(item Item, env EnvResolver) DetectionResult {
	if env == nil {
		env = defaultEnvResolver
	}

	result := DetectionResult{
		ID:    item.ID,
		Label: item.Label,
	}

	for _, cred := range item.Credentials {
		status := CredentialStatus{Label: cred.Label}
		for _, src := range cred.Sources {
			if desc, ok := detectSource(src, env); ok {
				status.Available = true
				status.SourceMatched = desc
				break
			}
		}
		if status.Available {
			result.Available = true
		}
		result.Credentials = append(result.Credentials, status)
	}

	return result
}

// resolveCredential tries each source in order and returns the first
// that resolves. If no source is detected, it returns a non-resolved
// result (not an error).
func resolveCredential(cred Credential, env EnvResolver) (*ResolvedCredential, error) {
	result := &ResolvedCredential{Label: cred.Label}

	for _, src := range cred.Sources {
		desc, value, ok := trySource(src, env)
		if !ok {
			continue
		}

		var extraInjections []ResolvedInjection
		if cred.Transform != nil {
			var err error
			value, extraInjections, err = applyTransform(value, *cred.Transform)
			if err != nil {
				result.Error = fmt.Sprintf("transform failed: %v", err)
				return result, nil
			}
		}

		result.Resolved = true
		result.SourceMatched = desc

		// Build the set of extra injection keys so we can skip
		// caller-produced injections that the transform overrides
		// (e.g. TransformGitInclude replaces the main gitconfig
		// mount with a content-bearing version).
		extraKeys := make(map[string]struct{}, len(extraInjections))
		for _, ei := range extraInjections {
			extraKeys[ei.Key] = struct{}{}
		}

		for _, inj := range cred.Injections {
			if _, overridden := extraKeys[inj.Key]; overridden {
				continue
			}
			result.Injections = append(result.Injections, buildInjection(inj, value))
		}
		result.Injections = append(result.Injections, extraInjections...)

		return result, nil
	}

	return result, nil
}

// trySource detects and reads a source in a single pass, avoiding the
// TOCTOU race and double execution that separate detect+read would cause
// (especially for command sources which fork a child process).
func trySource(src Source, env EnvResolver) (desc string, value string, ok bool) {
	switch src.Type {
	case SourceEnvVar:
		v, exists := env.LookupEnv(src.Value)
		if !exists {
			return "", "", false
		}
		return fmt.Sprintf("env $%s", src.Value), v, true

	case SourceFilePath:
		path := expandHome(src.Value)
		if _, err := os.Stat(path); err != nil {
			return "", "", false
		}
		// For file sources, the resolved value is the absolute host path
		// (used for bind mounts). File contents are not read.
		return fmt.Sprintf("file %s", src.Value), path, true

	case SourceSocketPath:
		path := env.ExpandEnv(src.Value)
		fi, err := os.Stat(path)
		if err != nil || fi.Mode().Type() != os.ModeSocket {
			return "", "", false
		}
		if !probeSocket(path) {
			return "", "", false
		}
		return fmt.Sprintf("socket %s", src.Value), path, true

	case SourceNamedPipe:
		if !probeNamedPipe(src.Value) {
			return "", "", false
		}
		return fmt.Sprintf("pipe %s", src.Value), src.Value, true

	case SourceCommand:
		name, args := parseCommand(src.Value)
		cmd := exec.Command(name, args...)
		cmd.Env = env.Environ()
		out, err := cmd.Output()
		if err != nil {
			return "", "", false
		}
		return fmt.Sprintf("command %q", src.Value), strings.TrimSpace(string(out)), true
	}

	return "", "", false
}

// detectSource checks whether a source is available on the host without
// reading its value. Used by [Detect] for lightweight availability checks.
func detectSource(src Source, env EnvResolver) (string, bool) {
	switch src.Type {
	case SourceEnvVar:
		if _, ok := env.LookupEnv(src.Value); ok {
			return fmt.Sprintf("env $%s", src.Value), true
		}
	case SourceFilePath:
		path := expandHome(src.Value)
		if _, err := os.Stat(path); err == nil {
			return fmt.Sprintf("file %s", src.Value), true
		}
	case SourceSocketPath:
		path := env.ExpandEnv(src.Value)
		if fi, err := os.Stat(path); err == nil && fi.Mode().Type() == os.ModeSocket && probeSocket(path) {
			return fmt.Sprintf("socket %s", src.Value), true
		}
	case SourceNamedPipe:
		if probeNamedPipe(src.Value) {
			return fmt.Sprintf("pipe %s", src.Value), true
		}
	case SourceCommand:
		name, args := parseCommand(src.Value)
		cmd := exec.Command(name, args...)
		cmd.Env = env.Environ()
		if err := cmd.Run(); err == nil {
			return fmt.Sprintf("command %q", src.Value), true
		}
	}
	return "", false
}

// applyTransform applies a built-in transformation to the resolved value.
// Returns the (possibly modified) value, any extra injections produced by
// the transform (e.g. additional file mounts), and an error.
func applyTransform(value string, t Transform) (string, []ResolvedInjection, error) {
	switch t.Type {
	case TransformStripLines:
		pattern, ok := t.Params["pattern"]
		if !ok {
			return "", nil, fmt.Errorf("strip_lines transform requires 'pattern' param")
		}
		re, err := regexp.Compile("(?i)" + pattern)
		if err != nil {
			return "", nil, fmt.Errorf("invalid strip_lines pattern: %w", err)
		}
		var buf bytes.Buffer
		for line := range strings.SplitSeq(value, "\n") {
			if !re.MatchString(line) {
				buf.WriteString(line)
				buf.WriteByte('\n')
			}
		}
		return strings.TrimRight(buf.String(), "\n"), nil, nil

	case TransformGitInclude:
		return applyGitIncludeTransform(value)

	default:
		return "", nil, fmt.Errorf("unknown transform type: %s", t.Type)
	}
}

// applyGitIncludeTransform reads the gitconfig file at the given host
// path, parses include/includeIf directives, and produces additional
// mount injections for each referenced file that exists on the host.
// The gitconfig content is rewritten so include paths point to
// container mount locations under [ContainerGitIncludeDir].
//
// If the file has no includes or cannot be read, the original host
// path is returned unchanged (backward compatible).
//
// The returned extra injections include a content-bearing injection
// for the main gitconfig mount (with rewritten paths) and one mount
// injection per discovered include file. resolveCredential deduplicates
// extras against the caller-produced injections by Key.
func applyGitIncludeTransform(hostPath string) (string, []ResolvedInjection, error) {
	content, err := os.ReadFile(hostPath)
	if err != nil {
		return hostPath, nil, nil
	}

	includePaths := ParseGitIncludePaths(string(content))
	if len(includePaths) == 0 {
		return hostPath, nil, nil
	}

	configDir := filepath.Dir(hostPath)
	pathMap := make(map[string]string, len(includePaths))
	var includeInjections []ResolvedInjection
	usedNames := make(map[string]int)

	for _, rawPath := range includePaths {
		resolvedHostPath := ResolveIncludePath(rawPath, configDir)

		// EvalSymlinks resolves symlinks (Nix Home Manager creates
		// symlinks to /nix/store/...) and implicitly verifies the
		// target exists — it returns an error if any component of
		// the path is missing.
		real, evalErr := filepath.EvalSymlinks(resolvedHostPath)
		if evalErr != nil {
			continue
		}
		resolvedHostPath = real

		// Build a unique container-side name, disambiguating
		// basename collisions with a numeric suffix.
		base := filepath.Base(resolvedHostPath)
		containerPath := ContainerGitIncludePath(base)
		if count, exists := usedNames[base]; exists {
			containerPath = ContainerGitIncludePath(fmt.Sprintf("%s.%d", base, count))
			usedNames[base] = count + 1
		} else {
			usedNames[base] = 1
		}

		pathMap[rawPath] = containerPath
		includeInjections = append(includeInjections, ResolvedInjection{
			Type:     InjectionMountFile,
			Key:      containerPath,
			Value:    resolvedHostPath,
			ReadOnly: true,
		})
	}

	if len(pathMap) == 0 {
		return hostPath, nil, nil
	}

	rewritten := RewriteGitIncludePaths(string(content), pathMap)

	// Build the full extras list: a content-bearing replacement for
	// the main gitconfig mount, plus one mount per include file.
	extras := make([]ResolvedInjection, 0, 1+len(includeInjections))
	extras = append(extras, ResolvedInjection{
		Type:     InjectionMountFile,
		Key:      ContainerGitConfigHostPath,
		Value:    hostPath,
		ReadOnly: true,
		Content:  rewritten,
	})
	extras = append(extras, includeInjections...)

	return hostPath, extras, nil
}

// buildInjection constructs a ResolvedInjection from the injection spec
// and resolved value. If the injection has a static Value override, that
// takes precedence over the resolved source value.
func buildInjection(inj Injection, resolvedValue string) ResolvedInjection {
	value := resolvedValue
	if inj.Value != "" {
		value = inj.Value
	}

	return ResolvedInjection{
		Type:     inj.Type,
		Key:      inj.Key,
		Value:    value,
		ReadOnly: inj.ReadOnly,
	}
}

// probeSocket attempts a TCP-less connection to a Unix domain socket to
// verify it has a live listener. Stale sockets (process exited, systemd
// unit stopped) remain on disk as regular socket files but have no
// listener — os.Stat passes but the mount will fail at container
// creation time. A quick dial catches this early.
func probeSocket(path string) bool {
	conn, err := net.DialTimeout("unix", path, ProbeTimeout)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// expandHome replaces a leading ~ with the user's home directory.
func expandHome(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home := homeDir()
	if home == "" {
		return path
	}
	return home + path[1:]
}

// parseCommand splits a command string into the executable name and arguments.
func parseCommand(cmd string) (string, []string) {
	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return "", nil
	}
	return parts[0], parts[1:]
}

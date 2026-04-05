package access

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// defaultShellEnvTimeout is the maximum time to wait for the user's
// login shell to print its environment.
const defaultShellEnvTimeout = 10 * time.Second

// defaultRefreshCooldown is the minimum interval between shell env
// refreshes. Rapid Test-button clicks reuse the cache.
const defaultRefreshCooldown = 30 * time.Second

// validEnvKey matches valid POSIX environment variable names.
var validEnvKey = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// ShellEnvResolver captures the user's full shell environment by
// spawning their login shell. This allows Warden to see env vars
// defined in .bashrc, .zshrc, .profile, etc. even when launched
// from a desktop entry that doesn't inherit the shell environment.
//
// Process env always takes precedence over the shell cache — if
// a variable is set in both, the process value wins.
type ShellEnvResolver struct {
	mu       sync.RWMutex
	cache    map[string]string
	loadedAt time.Time
	timeout  time.Duration
	cooldown time.Duration
}

// NewShellEnvResolver creates a resolver that will spawn the user's
// login shell to capture environment variables. Call [Load] (typically
// in a background goroutine) to populate the cache eagerly at startup.
func NewShellEnvResolver() *ShellEnvResolver {
	return &ShellEnvResolver{
		timeout:  defaultShellEnvTimeout,
		cooldown: defaultRefreshCooldown,
	}
}

// Load spawns the user's login shell and caches the resulting
// environment. Safe to call from a goroutine. If the shell fails
// or times out, the resolver degrades gracefully to process-only env.
func (r *ShellEnvResolver) Load() error {
	env, err := spawnShell(r.timeout)

	r.mu.Lock()
	defer r.mu.Unlock()

	r.cache = env
	r.loadedAt = time.Now()

	if err != nil {
		return fmt.Errorf("loading shell env: %w", err)
	}
	return nil
}

// Refresh re-spawns the login shell to pick up any env changes since
// the last load. Skips the spawn if the cache was loaded less than
// 30 seconds ago (cooldown).
func (r *ShellEnvResolver) Refresh() error {
	r.mu.RLock()
	age := time.Since(r.loadedAt)
	r.mu.RUnlock()

	if age < r.cooldown {
		return nil
	}

	return r.Load()
}

// LookupEnv checks the process environment first, then falls back
// to the cached shell environment. Process env takes precedence
// because explicitly set variables are the most intentional.
func (r *ShellEnvResolver) LookupEnv(key string) (string, bool) {
	if v, ok := os.LookupEnv(key); ok {
		return v, true
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.cache != nil {
		if v, ok := r.cache[key]; ok {
			return v, true
		}
	}
	return "", false
}

// ExpandEnv replaces ${var} and $var in the string using the combined
// process + shell environment.
func (r *ShellEnvResolver) ExpandEnv(s string) string {
	return os.Expand(s, func(key string) string {
		v, _ := r.LookupEnv(key)
		return v
	})
}

// Environ returns the full combined environment as a []string slice
// (KEY=VALUE format). Shell cache is the base; process env is overlaid
// so process values always win on conflicts.
func (r *ShellEnvResolver) Environ() []string {
	r.mu.RLock()
	merged := make(map[string]string, len(r.cache))
	for k, v := range r.cache {
		merged[k] = v
	}
	r.mu.RUnlock()

	// Process env wins on conflicts.
	for _, entry := range os.Environ() {
		k, v, ok := strings.Cut(entry, "=")
		if ok {
			merged[k] = v
		}
	}

	result := make([]string, 0, len(merged))
	for k, v := range merged {
		result = append(result, k+"="+v)
	}
	return result
}

// spawnShell runs the user's login shell and captures its environment.
// Returns nil map (not error) on Windows where desktop env inheritance
// works via the registry.
func spawnShell(timeout time.Duration) (map[string]string, error) {
	if runtime.GOOS == "windows" {
		return nil, nil
	}

	shell := resolveShell()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, shell, "-ilc", "env")
	cmd.Stdin = nil         // stdin closed — prevents interactive prompts
	cmd.Stderr = io.Discard // suppress shell startup messages from reaching the terminal
	configureShellCmd(cmd)  // isolate from the terminal to prevent SIGTTOU races

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("running %s -ilc env: %w", shell, err)
	}

	env := parseShellEnvOutput(out)
	slog.Debug("loaded shell environment", "shell", shell, "vars", len(env))
	return env, nil
}

// resolveShell returns the user's shell, falling back to common defaults.
func resolveShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	if _, err := exec.LookPath("bash"); err == nil {
		return "bash"
	}
	return "/bin/sh"
}

// parseShellEnvOutput parses the output of `env` into a key-value map.
// Lines that don't match KEY=VALUE format (prompt noise, MOTD, escape
// sequences) are silently skipped.
func parseShellEnvOutput(data []byte) map[string]string {
	env := make(map[string]string)
	for _, line := range strings.Split(string(data), "\n") {
		k, v, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		if !validEnvKey.MatchString(k) {
			continue
		}
		env[k] = v
	}
	return env
}

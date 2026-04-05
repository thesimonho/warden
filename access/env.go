package access

import "os"

// EnvResolver abstracts environment variable lookup so callers can
// provide a combined process + shell environment. This allows Warden
// to resolve credentials from shell config files (.bashrc, .zshrc,
// .profile) even when launched from a desktop entry that doesn't
// inherit the user's shell environment.
type EnvResolver interface {
	// LookupEnv returns the value of the named environment variable.
	LookupEnv(key string) (string, bool)

	// ExpandEnv replaces ${var} and $var references in the string
	// using the resolver's environment.
	ExpandEnv(s string) string

	// Environ returns the full environment as a []string slice
	// (KEY=VALUE format), suitable for use as exec.Cmd.Env.
	Environ() []string
}

// Refresher is implemented by EnvResolver implementations that support
// refreshing their cached environment (e.g. by re-spawning the user's
// login shell). The service layer type-asserts against this interface
// before Test and container-create operations.
type Refresher interface {
	Refresh() error
}

// ProcessEnvResolver delegates directly to the os package. It is the
// default resolver used when no shell environment is available, and
// is the test-friendly implementation (works with t.Setenv).
type ProcessEnvResolver struct{}

// LookupEnv delegates to [os.LookupEnv].
func (ProcessEnvResolver) LookupEnv(key string) (string, bool) {
	return os.LookupEnv(key)
}

// ExpandEnv delegates to [os.ExpandEnv].
func (ProcessEnvResolver) ExpandEnv(s string) string {
	return os.ExpandEnv(s)
}

// Environ delegates to [os.Environ].
func (ProcessEnvResolver) Environ() []string {
	return os.Environ()
}

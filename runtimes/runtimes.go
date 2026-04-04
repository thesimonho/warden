// Package runtimes defines the language runtime registry for Warden containers.
//
// Each runtime declares what gets installed, which network domains it needs
// (for restricted network mode), which environment variables it sets (for
// cache persistence), and which marker files indicate a project uses it.
//
// The package has no internal dependencies — it is importable by any consumer.
package runtimes

import (
	"os"
	"slices"
	"strings"
)

// Runtime describes a language runtime that can be installed in a container.
type Runtime struct {
	// ID is the unique identifier (e.g. "node", "python", "go").
	ID string
	// Label is the human-readable name (e.g. "Node.js", "Python", "Go").
	Label string
	// Description briefly explains what gets installed.
	Description string
	// AlwaysEnabled means this runtime cannot be deselected (e.g. Node for MCP).
	AlwaysEnabled bool
	// Domains lists network domains required for this runtime's package registry.
	Domains []string
	// EnvVars maps environment variable names to values set when this runtime
	// is enabled. These point caches to the shared volume for persistence.
	EnvVars map[string]string
	// MarkerFiles lists filenames whose presence in the project root indicates
	// the project uses this runtime.
	MarkerFiles []string
}

const cacheBase = "/home/warden/.cache/warden-runtimes"

// registry is the ordered list of available runtimes.
var registry = []Runtime{
	{
		ID:            "node",
		Label:         "Node.js",
		Description:   "Already installed. Required for MCP servers.",
		AlwaysEnabled: true,
		Domains: []string{
			"registry.npmjs.org",
			"registry.yarnpkg.com",
		},
		EnvVars: map[string]string{
			"npm_config_cache": cacheBase + "/npm",
		},
		MarkerFiles: nil,
	},
	{
		ID:          "python",
		Label:       "Python",
		Description: "Installs python3, pip, and venv.",
		Domains: []string{
			"pypi.org",
			"files.pythonhosted.org",
		},
		EnvVars: map[string]string{
			"PIP_CACHE_DIR": cacheBase + "/pip",
		},
		MarkerFiles: []string{
			"pyproject.toml",
			"requirements.txt",
			"setup.py",
			"Pipfile",
			"uv.lock",
			".python-version",
		},
	},
	{
		ID:          "go",
		Label:       "Go",
		Description: "Installs Go from go.dev. Reads version from go.mod.",
		Domains: []string{
			"go.dev",
			"dl.google.com",
			"proxy.golang.org",
			"sum.golang.org",
			"storage.googleapis.com",
		},
		EnvVars: map[string]string{
			"GOMODCACHE": cacheBase + "/go/mod",
			"GOPATH":     cacheBase + "/go/path",
		},
		MarkerFiles: []string{
			"go.mod",
		},
	},
	{
		ID:          "rust",
		Label:       "Rust",
		Description: "Installs rustup and cargo. Reads version from rust-toolchain.toml.",
		Domains: []string{
			"sh.rustup.rs",
			"static.rust-lang.org",
			"crates.io",
			"static.crates.io",
			"index.crates.io",
		},
		EnvVars: map[string]string{
			"CARGO_HOME":  cacheBase + "/cargo",
			"RUSTUP_HOME": cacheBase + "/rustup",
		},
		MarkerFiles: []string{
			"Cargo.toml",
			"rust-toolchain.toml",
		},
	},
	{
		ID:          "ruby",
		Label:       "Ruby",
		Description: "Installs ruby and bundler.",
		Domains: []string{
			"rubygems.org",
			"index.rubygems.org",
		},
		EnvVars: map[string]string{
			"GEM_SPEC_CACHE": cacheBase + "/gem",
		},
		MarkerFiles: []string{
			"Gemfile",
			".ruby-version",
		},
	},
	{
		ID:          "lua",
		Label:       "Lua",
		Description: "Installs lua and luarocks.",
		Domains: []string{
			"luarocks.org",
		},
		EnvVars: map[string]string{
			"LUAROCKS_CONFIG": cacheBase + "/luarocks/config.lua",
		},
		MarkerFiles: []string{
			".luarc.json",
			".luacheckrc",
		},
	},
}

// Registry returns the full ordered list of available runtimes.
func Registry() []Runtime {
	result := make([]Runtime, len(registry))
	copy(result, registry)
	return result
}

// ByID returns a runtime by its identifier, or nil if not found.
func ByID(id string) *Runtime {
	for i := range registry {
		if registry[i].ID == id {
			r := registry[i]
			return &r
		}
	}
	return nil
}

// Detect scans the project root directory for marker files and returns
// a map of runtime ID to detected status. Only performs a shallow scan
// (no recursion). Always-enabled runtimes are always marked as detected.
func Detect(projectPath string) map[string]bool {
	result := make(map[string]bool, len(registry))

	// Read directory once to avoid per-file os.Stat calls.
	files := readDirNames(projectPath)

	for _, r := range registry {
		if r.AlwaysEnabled {
			result[r.ID] = true
			continue
		}
		result[r.ID] = hasAnyMatch(files, r.MarkerFiles)
	}
	return result
}

// DomainsForRuntimes collects and deduplicates all network domains
// required by the given runtime IDs.
func DomainsForRuntimes(ids []string) []string {
	seen := make(map[string]bool)
	var domains []string
	for _, id := range ids {
		r := ByID(id)
		if r == nil {
			continue
		}
		for _, d := range r.Domains {
			if !seen[d] {
				seen[d] = true
				domains = append(domains, d)
			}
		}
	}
	return domains
}

// EnvVarsForRuntimes collects all environment variables for the given
// runtime IDs. Later runtimes override earlier ones if keys conflict.
func EnvVarsForRuntimes(ids []string) map[string]string {
	result := make(map[string]string)
	for _, id := range ids {
		r := ByID(id)
		if r == nil {
			continue
		}
		for k, v := range r.EnvVars {
			result[k] = v
		}
	}
	return result
}

// AllIDs returns the IDs of all registered runtimes in registry order.
func AllIDs() []string {
	ids := make([]string, len(registry))
	for i, r := range registry {
		ids[i] = r.ID
	}
	return ids
}

// IsValidID reports whether the given string is a registered runtime ID.
func IsValidID(id string) bool {
	return slices.ContainsFunc(registry, func(r Runtime) bool {
		return r.ID == id
	})
}

// DomainsByRuntime returns a map of runtime ID to its network domains,
// for all runtimes in the given list. Useful for displaying which
// runtime contributed which domains.
func DomainsByRuntime(ids []string) map[string][]string {
	result := make(map[string][]string, len(ids))
	for _, id := range ids {
		r := ByID(id)
		if r == nil {
			continue
		}
		if len(r.Domains) > 0 {
			domains := make([]string, len(r.Domains))
			copy(domains, r.Domains)
			result[id] = domains
		}
	}
	return result
}

// FilterUserDomains removes any domains that are contributed by the given
// runtimes from the domain list, returning only user-specified domains.
func FilterUserDomains(allDomains []string, runtimeIDs []string) []string {
	runtimeDomains := make(map[string]bool)
	for _, d := range DomainsForRuntimes(runtimeIDs) {
		runtimeDomains[strings.ToLower(d)] = true
	}

	var userDomains []string
	for _, d := range allDomains {
		if !runtimeDomains[strings.ToLower(d)] {
			userDomains = append(userDomains, d)
		}
	}
	return userDomains
}

// readDirNames returns the set of filenames in a directory for fast lookup.
func readDirNames(dir string) map[string]bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	names := make(map[string]bool, len(entries))
	for _, e := range entries {
		names[e.Name()] = true
	}
	return names
}

// hasAnyMatch checks whether any of the given filenames exist in the name set.
func hasAnyMatch(dirNames map[string]bool, filenames []string) bool {
	for _, f := range filenames {
		if dirNames[f] {
			return true
		}
	}
	return false
}

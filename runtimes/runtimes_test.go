package runtimes

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRegistry(t *testing.T) {
	t.Parallel()
	reg := Registry()
	if len(reg) == 0 {
		t.Fatal("registry should not be empty")
	}

	ids := make(map[string]bool)
	for _, r := range reg {
		if r.ID == "" {
			t.Error("runtime has empty ID")
		}
		if ids[r.ID] {
			t.Errorf("duplicate runtime ID: %s", r.ID)
		}
		ids[r.ID] = true

		if r.Label == "" {
			t.Errorf("runtime %s has empty Label", r.ID)
		}
		if r.Description == "" {
			t.Errorf("runtime %s has empty Description", r.ID)
		}
	}

	// Verify node is always enabled.
	node := ByID("node")
	if node == nil {
		t.Fatal("node runtime not found")
	}
	if !node.AlwaysEnabled {
		t.Error("node should be always enabled")
	}
}

func TestByID(t *testing.T) {
	t.Parallel()
	if r := ByID("go"); r == nil || r.ID != "go" {
		t.Error("expected to find go runtime")
	}
	if r := ByID("nonexistent"); r != nil {
		t.Error("expected nil for nonexistent runtime")
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create marker files.
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[project]\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	result := Detect(dir)

	if !result["node"] {
		t.Error("node should always be detected")
	}
	if !result["go"] {
		t.Error("go should be detected (go.mod present)")
	}
	if !result["python"] {
		t.Error("python should be detected (pyproject.toml present)")
	}
	if result["rust"] {
		t.Error("rust should not be detected (no Cargo.toml)")
	}
	if result["ruby"] {
		t.Error("ruby should not be detected (no Gemfile)")
	}
}

func TestDetectEmptyDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := Detect(dir)
	if !result["node"] {
		t.Error("node should always be detected")
	}
	for _, id := range []string{"python", "go", "rust", "ruby", "lua"} {
		if result[id] {
			t.Errorf("%s should not be detected in empty dir", id)
		}
	}
}

func TestDomainsForRuntimes(t *testing.T) {
	t.Parallel()
	domains := DomainsForRuntimes([]string{"python", "go"})
	if len(domains) == 0 {
		t.Fatal("expected domains for python and go")
	}

	has := make(map[string]bool)
	for _, d := range domains {
		has[d] = true
	}
	if !has["pypi.org"] {
		t.Error("expected pypi.org in domains")
	}
	if !has["proxy.golang.org"] {
		t.Error("expected proxy.golang.org in domains")
	}
}

func TestDomainsForRuntimesDeduplicates(t *testing.T) {
	t.Parallel()
	// Passing the same runtime twice should not duplicate domains.
	domains := DomainsForRuntimes([]string{"python", "python"})
	seen := make(map[string]int)
	for _, d := range domains {
		seen[d]++
		if seen[d] > 1 {
			t.Errorf("duplicate domain: %s", d)
		}
	}
}

func TestEnvVarsForRuntimes(t *testing.T) {
	t.Parallel()
	envs := EnvVarsForRuntimes([]string{"go", "python"})
	if envs["GOMODCACHE"] == "" {
		t.Error("expected GOMODCACHE to be set")
	}
	if envs["PIP_CACHE_DIR"] == "" {
		t.Error("expected PIP_CACHE_DIR to be set")
	}
}

func TestFilterUserDomains(t *testing.T) {
	t.Parallel()
	allDomains := []string{"pypi.org", "custom.example.com", "files.pythonhosted.org"}
	userOnly := FilterUserDomains(allDomains, []string{"python"})
	if len(userOnly) != 1 || userOnly[0] != "custom.example.com" {
		t.Errorf("expected only custom.example.com, got %v", userOnly)
	}
}

func TestDomainsByRuntime(t *testing.T) {
	t.Parallel()
	result := DomainsByRuntime([]string{"python", "go"})
	if len(result["python"]) == 0 {
		t.Error("expected python domains")
	}
	if len(result["go"]) == 0 {
		t.Error("expected go domains")
	}
}

func TestSystemDomains(t *testing.T) {
	t.Parallel()
	domains := SystemDomains()
	if len(domains) == 0 {
		t.Fatal("expected at least one system domain")
	}

	has := make(map[string]bool)
	for _, d := range domains {
		has[d] = true
	}
	if !has["storage.googleapis.com"] {
		t.Error("expected storage.googleapis.com for Claude CLI downloads")
	}
}

func TestIsValidID(t *testing.T) {
	t.Parallel()
	if !IsValidID("node") {
		t.Error("node should be valid")
	}
	if !IsValidID("lua") {
		t.Error("lua should be valid")
	}
	if IsValidID("nonexistent") {
		t.Error("nonexistent should not be valid")
	}
}

func TestAlwaysEnabledIDs(t *testing.T) {
	t.Parallel()
	ids := AlwaysEnabledIDs()
	if len(ids) == 0 {
		t.Fatal("expected at least one always-enabled runtime (node)")
	}
	if ids[0] != "node" {
		t.Errorf("expected node as first always-enabled runtime, got %q", ids[0])
	}
	// Verify all returned IDs are actually always-enabled.
	for _, id := range ids {
		r := ByID(id)
		if r == nil || !r.AlwaysEnabled {
			t.Errorf("runtime %q returned by AlwaysEnabledIDs but is not always-enabled", id)
		}
	}
}

func TestAllIDs(t *testing.T) {
	t.Parallel()
	ids := AllIDs()
	if len(ids) != len(registry) {
		t.Errorf("expected %d IDs, got %d", len(registry), len(ids))
	}
	if ids[0] != "node" {
		t.Error("first ID should be node")
	}
}

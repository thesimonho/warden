package service

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
)

func TestReadProjectTemplate(t *testing.T) {
	t.Parallel()

	t.Run("valid template", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		skipPerms := true
		budget := 5.0
		tmpl := api.ProjectTemplate{
			Image:           "custom:latest",
			SkipPermissions: &skipPerms,
			NetworkMode:     api.NetworkModeRestricted,
			CostBudget:      &budget,
			Runtimes:        []string{"node", "python"},
			Agents: map[string]api.AgentTemplateOverride{
				"claude-code": {AllowedDomains: []string{"*.anthropic.com"}},
			},
		}
		writeJSON(t, dir, tmpl)

		result := readProjectTemplate(dir)
		if result == nil {
			t.Fatal("expected template, got nil")
		}
		if result.Image != "custom:latest" {
			t.Errorf("expected image 'custom:latest', got %q", result.Image)
		}
		if result.SkipPermissions == nil || !*result.SkipPermissions {
			t.Error("expected skipPermissions to be true")
		}
		if result.CostBudget == nil || *result.CostBudget != 5.0 {
			t.Error("expected costBudget to be 5.0")
		}
		if len(result.Runtimes) != 2 {
			t.Errorf("expected 2 runtimes, got %d", len(result.Runtimes))
		}
		domains := result.Agents["claude-code"].AllowedDomains
		if len(domains) != 1 || domains[0] != "*.anthropic.com" {
			t.Errorf("expected [*.anthropic.com], got %v", domains)
		}
	})

	t.Run("missing file returns nil", func(t *testing.T) {
		t.Parallel()
		result := readProjectTemplate(t.TempDir())
		if result != nil {
			t.Error("expected nil for missing file")
		}
	})

	t.Run("invalid JSON returns nil", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		_ = os.WriteFile(filepath.Join(dir, templateFileName), []byte("not json"), 0o644)

		result := readProjectTemplate(dir)
		if result != nil {
			t.Error("expected nil for invalid JSON")
		}
	})

	t.Run("security: domains stripped when networkMode is not restricted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		tmpl := api.ProjectTemplate{
			NetworkMode: api.NetworkModeNone,
			Agents: map[string]api.AgentTemplateOverride{
				"claude-code": {AllowedDomains: []string{"*"}},
			},
		}
		writeJSON(t, dir, tmpl)

		result := readProjectTemplate(dir)
		if result == nil {
			t.Fatal("expected template, got nil")
		}
		if _, ok := result.Agents["claude-code"]; ok {
			t.Error("expected agent override to be deleted when networkMode is not restricted")
		}
	})
}

func TestNewTemplateData(t *testing.T) {
	t.Parallel()

	t.Run("defaults empty agent type", func(t *testing.T) {
		t.Parallel()
		td := newTemplateData(api.CreateContainerRequest{AgentType: ""})
		if td.AgentType != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", td.AgentType)
		}
	})

	t.Run("preserves explicit agent type", func(t *testing.T) {
		t.Parallel()
		td := newTemplateData(api.CreateContainerRequest{AgentType: "codex"})
		if td.AgentType != "codex" {
			t.Errorf("expected 'codex', got %q", td.AgentType)
		}
	})

	t.Run("defaults empty runtimes to always-enabled", func(t *testing.T) {
		t.Parallel()
		td := newTemplateData(api.CreateContainerRequest{EnabledRuntimes: nil})
		if len(td.Runtimes) == 0 {
			t.Fatal("expected non-empty runtimes")
		}
		if td.Runtimes[0] != "node" {
			t.Errorf("expected 'node', got %q", td.Runtimes[0])
		}
	})

	t.Run("preserves explicit runtimes", func(t *testing.T) {
		t.Parallel()
		td := newTemplateData(api.CreateContainerRequest{
			EnabledRuntimes: []string{"node", "python", "go"},
		})
		if len(td.Runtimes) != 3 {
			t.Errorf("expected 3 runtimes, got %d", len(td.Runtimes))
		}
	})

	t.Run("copies all fields", func(t *testing.T) {
		t.Parallel()
		td := newTemplateData(api.CreateContainerRequest{
			ProjectPath:     "/home/user/project",
			Image:           "custom:latest",
			AgentType:       constants.AgentClaudeCode,
			SkipPermissions: true,
			NetworkMode:     api.NetworkModeRestricted,
			CostBudget:      25.0,
			EnabledRuntimes: []string{"node", "python"},
			AllowedDomains:  []string{"*.anthropic.com"},
		})
		if td.ProjectPath != "/home/user/project" {
			t.Errorf("ProjectPath = %q", td.ProjectPath)
		}
		if td.Image != "custom:latest" {
			t.Errorf("Image = %q", td.Image)
		}
		if !td.SkipPermissions {
			t.Error("expected SkipPermissions=true")
		}
		if td.NetworkMode != api.NetworkModeRestricted {
			t.Errorf("NetworkMode = %q", td.NetworkMode)
		}
		if td.CostBudget != 25.0 {
			t.Errorf("CostBudget = %f", td.CostBudget)
		}
		if len(td.AllowedDomains) != 1 {
			t.Errorf("expected 1 domain, got %d", len(td.AllowedDomains))
		}
	})
}

func TestWriteProjectTemplate(t *testing.T) {
	t.Parallel()

	t.Run("writes correct structure", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		td := newTemplateData(api.CreateContainerRequest{
			Image:           "custom:latest",
			ProjectPath:     dir,
			AgentType:       constants.AgentClaudeCode,
			SkipPermissions: true,
			NetworkMode:     api.NetworkModeRestricted,
			AllowedDomains:  []string{"*.anthropic.com", "*.github.com"},
			CostBudget:      10.0,
			EnabledRuntimes: []string{"node", "python"},
		})

		writeProjectTemplate(td)

		data, err := os.ReadFile(filepath.Join(dir, templateFileName))
		if err != nil {
			t.Fatalf("failed to read written template: %v", err)
		}

		var tmpl api.ProjectTemplate
		if err := json.Unmarshal(data, &tmpl); err != nil {
			t.Fatalf("failed to parse written template: %v", err)
		}

		if tmpl.Image != "custom:latest" {
			t.Errorf("expected image 'custom:latest', got %q", tmpl.Image)
		}
		if tmpl.SkipPermissions == nil || !*tmpl.SkipPermissions {
			t.Error("expected skipPermissions to be true")
		}
		if len(tmpl.Runtimes) != 2 || tmpl.Runtimes[0] != "node" || tmpl.Runtimes[1] != "python" {
			t.Errorf("expected [node python], got %v", tmpl.Runtimes)
		}
		domains := tmpl.Agents["claude-code"].AllowedDomains
		if len(domains) != 2 {
			t.Errorf("expected 2 domains, got %v", domains)
		}
	})

	t.Run("preserves other agent overrides", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()

		// Write initial template with codex domains.
		initial := api.ProjectTemplate{
			Agents: map[string]api.AgentTemplateOverride{
				"codex": {AllowedDomains: []string{"*.openai.com"}},
			},
		}
		writeJSON(t, dir, initial)

		// Write-back as claude-code.
		td := newTemplateData(api.CreateContainerRequest{
			ProjectPath:    dir,
			AgentType:      constants.AgentClaudeCode,
			NetworkMode:    api.NetworkModeRestricted,
			AllowedDomains: []string{"*.anthropic.com"},
		})
		writeProjectTemplate(td)

		data, _ := os.ReadFile(filepath.Join(dir, templateFileName))
		var tmpl api.ProjectTemplate
		_ = json.Unmarshal(data, &tmpl)

		// Codex domains should be preserved.
		codexDomains := tmpl.Agents["codex"].AllowedDomains
		if len(codexDomains) != 1 || codexDomains[0] != "*.openai.com" {
			t.Errorf("expected codex domains preserved, got %v", codexDomains)
		}
		// Claude domains should be updated.
		claudeDomains := tmpl.Agents["claude-code"].AllowedDomains
		if len(claudeDomains) != 1 || claudeDomains[0] != "*.anthropic.com" {
			t.Errorf("expected claude domains, got %v", claudeDomains)
		}
	})

	t.Run("does not write domains when networkMode is not restricted", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		td := newTemplateData(api.CreateContainerRequest{
			ProjectPath:    dir,
			AgentType:      constants.AgentClaudeCode,
			NetworkMode:    api.NetworkModeNone,
			AllowedDomains: []string{"should-not-appear"},
		})

		writeProjectTemplate(td)

		data, _ := os.ReadFile(filepath.Join(dir, templateFileName))
		var tmpl api.ProjectTemplate
		_ = json.Unmarshal(data, &tmpl)

		if len(tmpl.Agents) > 0 {
			t.Errorf("expected no agents section when not restricted, got %v", tmpl.Agents)
		}
	})
}

func TestNormalizeHelpers(t *testing.T) {
	t.Parallel()

	t.Run("normalizeAgentType defaults empty", func(t *testing.T) {
		t.Parallel()
		if got := normalizeAgentType(""); got != "claude-code" {
			t.Errorf("expected 'claude-code', got %q", got)
		}
	})

	t.Run("normalizeAgentType preserves value", func(t *testing.T) {
		t.Parallel()
		if got := normalizeAgentType("codex"); got != "codex" {
			t.Errorf("expected 'codex', got %q", got)
		}
	})

	t.Run("normalizeRuntimes defaults empty", func(t *testing.T) {
		t.Parallel()
		got := normalizeRuntimes(nil)
		if len(got) == 0 || got[0] != "node" {
			t.Errorf("expected [node ...], got %v", got)
		}
	})

	t.Run("normalizeRuntimes preserves value", func(t *testing.T) {
		t.Parallel()
		got := normalizeRuntimes([]string{"python"})
		if len(got) != 1 || got[0] != "python" {
			t.Errorf("expected [python], got %v", got)
		}
	})

	t.Run("normalizeNetworkMode defaults empty", func(t *testing.T) {
		t.Parallel()
		if got := normalizeNetworkMode(""); got != api.NetworkModeFull {
			t.Errorf("expected 'full', got %q", got)
		}
	})

	t.Run("normalizeNetworkMode preserves value", func(t *testing.T) {
		t.Parallel()
		if got := normalizeNetworkMode(api.NetworkModeRestricted); got != api.NetworkModeRestricted {
			t.Errorf("expected 'restricted', got %q", got)
		}
	})
}

func TestReadProjectTemplateExported(t *testing.T) {
	t.Parallel()

	svc := New(ServiceDeps{DockerAvailable: true})

	t.Run("reads valid file", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		tmpl := api.ProjectTemplate{Image: "test:latest"}
		writeJSON(t, dir, tmpl)

		result, err := svc.ReadProjectTemplate(filepath.Join(dir, templateFileName))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result.Image != "test:latest" {
			t.Errorf("expected 'test:latest', got %q", result.Image)
		}
	})

	t.Run("returns error for missing file", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ReadProjectTemplate("/nonexistent/.warden.json")
		if err == nil {
			t.Error("expected error for missing file")
		}
	})

	t.Run("returns error for relative path", func(t *testing.T) {
		t.Parallel()
		_, err := svc.ReadProjectTemplate("relative/.warden.json")
		if err == nil {
			t.Error("expected error for relative path")
		}
	})
}

// writeJSON is a test helper that writes a ProjectTemplate to .warden.json.
func writeJSON(t *testing.T, dir string, tmpl api.ProjectTemplate) {
	t.Helper()
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		t.Fatalf("failed to marshal template: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, templateFileName), data, 0o644); err != nil {
		t.Fatalf("failed to write template: %v", err)
	}
}

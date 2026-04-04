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
			Runtimes: []string{"node", "python"},
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

func TestWriteProjectTemplate(t *testing.T) {
	t.Parallel()

	t.Run("writes correct structure", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		req := api.CreateContainerRequest{
			Image:              "custom:latest",
			ProjectPath:        dir,
			AgentType:          constants.AgentClaudeCode,
			SkipPermissions:    true,
			NetworkMode:        api.NetworkModeRestricted,
			AllowedDomains:     []string{"*.anthropic.com", "*.github.com"},
			CostBudget:      10.0,
			EnabledRuntimes: []string{"node", "python"},
		}

		writeProjectTemplate(dir, req)

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
		// Domains should be written for the current agent type.
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
		req := api.CreateContainerRequest{
			ProjectPath:    dir,
			AgentType:      constants.AgentClaudeCode,
			NetworkMode:    api.NetworkModeRestricted,
			AllowedDomains: []string{"*.anthropic.com"},
		}
		writeProjectTemplate(dir, req)

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
		req := api.CreateContainerRequest{
			ProjectPath:    dir,
			AgentType:      constants.AgentClaudeCode,
			NetworkMode:    api.NetworkModeNone,
			AllowedDomains: []string{"should-not-appear"},
		}

		writeProjectTemplate(dir, req)

		data, _ := os.ReadFile(filepath.Join(dir, templateFileName))
		var tmpl api.ProjectTemplate
		_ = json.Unmarshal(data, &tmpl)

		if len(tmpl.Agents) > 0 {
			t.Errorf("expected no agents section when not restricted, got %v", tmpl.Agents)
		}
	})
}

func TestReadProjectTemplateExported(t *testing.T) {
	t.Parallel()

	svc := New(ServiceDeps{})

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

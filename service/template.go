package service

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/runtimes"
)

// templateFileName is the well-known name for project template files.
const templateFileName = ".warden.json"

// --- Normalize helpers ---
//
// Single source of defaults for fields that can be empty in requests
// but need deterministic values for comparison, persistence, and template
// writes. Used by needsRecreation, auditContainerUpdate,
// projectRowFromRequest, and newTemplateData.

// normalizeAgentType returns the agent type string, defaulting to the
// default agent when empty.
func normalizeAgentType(at string) string {
	if at == "" {
		return string(agent.DefaultType)
	}
	return at
}

// normalizeRuntimes returns the runtime IDs, defaulting to always-enabled
// runtimes (Node.js) when the list is empty.
func normalizeRuntimes(ids []string) []string {
	if len(ids) == 0 {
		return runtimes.AlwaysEnabledIDs()
	}
	return ids
}

// normalizeNetworkMode returns the network mode, defaulting to full when empty.
func normalizeNetworkMode(nm api.NetworkMode) api.NetworkMode {
	if nm == "" {
		return api.NetworkModeFull
	}
	return nm
}

// --- Template read ---

// readProjectTemplate reads a .warden.json file from the given directory.
// Returns nil when the file does not exist or contains invalid JSON.
func readProjectTemplate(projectPath string) *api.ProjectTemplate {
	tmpl, err := parseTemplate(filepath.Join(projectPath, templateFileName))
	if err != nil {
		return nil
	}
	return tmpl
}

// parseTemplate reads and parses a .warden.json file, applying security
// filtering. Shared by both the directory-based and path-based readers.
func parseTemplate(filePath string) (*api.ProjectTemplate, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	var tmpl api.ProjectTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parsing template: %w", err)
	}

	sanitizeTemplate(&tmpl)
	return &tmpl, nil
}

// isEmptyTemplate returns true when no recognized ProjectTemplate fields are set.
func isEmptyTemplate(tmpl *api.ProjectTemplate) bool {
	return tmpl.Image == "" &&
		tmpl.SkipPermissions == nil &&
		tmpl.NetworkMode == "" &&
		tmpl.CostBudget == nil &&
		len(tmpl.Runtimes) == 0 &&
		len(tmpl.ForwardedPorts) == 0 &&
		len(tmpl.Agents) == 0
}

// sanitizeTemplate applies security and portability filters to a loaded template.
func sanitizeTemplate(tmpl *api.ProjectTemplate) {
	// Discard domains when network mode is not restricted to prevent
	// hidden permissive domain lists from being applied.
	if tmpl.NetworkMode != api.NetworkModeRestricted {
		for key := range tmpl.Agents {
			delete(tmpl.Agents, key)
		}
	}

	tmpl.Runtimes = filterValidRuntimes(tmpl.Runtimes)
}

// filterValidRuntimes returns only recognized runtime IDs.
func filterValidRuntimes(ids []string) []string {
	if len(ids) == 0 {
		return ids
	}
	filtered := make([]string, 0, len(ids))
	for _, id := range ids {
		if runtimes.IsValidID(id) {
			filtered = append(filtered, id)
		}
	}
	return filtered
}

// ReadProjectTemplate reads a .warden.json from an arbitrary file path.
// Unlike readProjectTemplate, this returns an error since the user
// explicitly requested the import.
func (s *Service) ReadProjectTemplate(filePath string) (*api.ProjectTemplate, error) {
	if !filepath.IsAbs(filePath) {
		return nil, fmt.Errorf("path must be absolute: %s", filePath)
	}
	return parseTemplate(filePath)
}

// ValidateProjectTemplate unmarshals and sanitizes a raw JSON template body.
// Used by the import-from-file flow where the frontend sends the file
// contents rather than a host path.
func (s *Service) ValidateProjectTemplate(data []byte) (*api.ProjectTemplate, error) {
	var tmpl api.ProjectTemplate
	if err := json.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if isEmptyTemplate(&tmpl) {
		return nil, fmt.Errorf("file does not contain any recognized .warden.json fields")
	}
	sanitizeTemplate(&tmpl)
	return &tmpl, nil
}

// --- Template write ---

// templateData is the normalized, defaulted input for writing .warden.json.
// All fields are guaranteed non-zero after construction via newTemplateData.
// This is intentionally narrower than CreateContainerRequest — it only
// contains the fields that belong in a portable project template. Fields
// excluded for security: envVars (may contain secrets), accessItems
// (resolve to credentials).
type templateData struct {
	ProjectPath     string
	Image           string
	AgentType       string
	SkipPermissions bool
	NetworkMode     api.NetworkMode
	CostBudget      float64
	Runtimes        []string
	AllowedDomains  []string
	ForwardedPorts  []int
}

// newTemplateData normalizes a CreateContainerRequest into a templateData.
// This is the single place where all template defaults are applied.
func newTemplateData(req api.CreateContainerRequest) templateData {
	return templateData{
		ProjectPath:     req.ProjectPath,
		Image:           req.Image,
		AgentType:       normalizeAgentType(string(req.AgentType)),
		SkipPermissions: req.SkipPermissions,
		NetworkMode:     normalizeNetworkMode(req.NetworkMode),
		CostBudget:      req.CostBudget,
		Runtimes:        normalizeRuntimes(req.EnabledRuntimes),
		AllowedDomains:  req.AllowedDomains,
		ForwardedPorts:  req.ForwardedPorts,
	}
}

// writeProjectTemplate writes the current container config back to
// .warden.json in the project directory. Preserves agent overrides for
// other agent types from the existing file. Best-effort: failures are
// logged but do not affect the create/update operation.
func writeProjectTemplate(td templateData) {
	templatePath := filepath.Join(td.ProjectPath, templateFileName)

	// Read existing file to preserve other agent overrides.
	var existing api.ProjectTemplate
	if data, err := os.ReadFile(templatePath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}

	tmpl := api.ProjectTemplate{
		Image:          td.Image,
		NetworkMode:    td.NetworkMode,
		Runtimes:       td.Runtimes,
		ForwardedPorts: td.ForwardedPorts,
	}

	// Pointer fields only set when non-default to keep the file clean.
	if td.SkipPermissions {
		skipPerms := true
		tmpl.SkipPermissions = &skipPerms
	}
	if td.CostBudget > 0 {
		budget := td.CostBudget
		tmpl.CostBudget = &budget
	}

	// Preserve existing agent overrides, update only the current agent.
	agents := existing.Agents
	if agents == nil {
		agents = make(map[string]api.AgentTemplateOverride)
	}
	if td.NetworkMode == api.NetworkModeRestricted && len(td.AllowedDomains) > 0 {
		agents[td.AgentType] = api.AgentTemplateOverride{
			AllowedDomains: td.AllowedDomains,
		}
	} else {
		delete(agents, td.AgentType)
	}
	if len(agents) > 0 {
		tmpl.Agents = agents
	}

	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		slog.Warn("failed to marshal .warden.json", "err", err)
		return
	}
	data = append(data, '\n')

	// Write atomically via temp file + rename.
	tmpFile := templatePath + ".tmp"
	if err := os.WriteFile(tmpFile, data, 0o644); err != nil {
		slog.Warn("failed to write .warden.json", "path", templatePath, "err", err)
		return
	}
	if err := os.Rename(tmpFile, templatePath); err != nil {
		slog.Warn("failed to rename .warden.json", "path", templatePath, "err", err)
		_ = os.Remove(tmpFile)
	}
}

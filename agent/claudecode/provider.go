// Package claudecode implements the agent.StatusProvider interface for
// Claude Code CLI. It reads session data from Claude Code's global config
// file (~/.claude.json) which stores per-project usage metrics.
package claudecode

import (
	"encoding/json"

	"github.com/thesimonho/warden/agent"
)

// configFilePath is the location of Claude Code's global config inside the container.
// Claude Code writes its config to ~/.claude.json (home directory root), NOT to
// ~/.claude/.claude.json (which is the bind-mounted host config).
const configFilePath = "/home/dev/.claude.json"

// Provider extracts status data from Claude Code's .claude.json config file.
type Provider struct{}

// NewProvider creates a new Claude Code status provider.
func NewProvider() *Provider {
	return &Provider{}
}

// Name returns the agent identifier.
func (p *Provider) Name() string {
	return "claude-code"
}

// ConfigFilePath returns the path to .claude.json inside the container.
func (p *Provider) ConfigFilePath() string {
	return configFilePath
}

// ExtractStatus parses .claude.json and returns status data keyed by
// the working directory path (which maps to our session worktree paths).
func (p *Provider) ExtractStatus(configData []byte) map[string]*agent.Status {
	return ParseConfig(configData)
}

// ParseConfig parses Claude Code's .claude.json and extracts per-project
// status data. Exported for testing.
func ParseConfig(data []byte) map[string]*agent.Status {
	if len(data) == 0 {
		return map[string]*agent.Status{}
	}

	var config claudeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return map[string]*agent.Status{}
	}

	result := make(map[string]*agent.Status, len(config.Projects))
	for path, project := range config.Projects {
		status := projectToStatus(project)
		if status != nil {
			result[path] = status
		}
	}

	return result
}

// projectToStatus converts a Claude Code project entry to an agent.Status.
// Returns nil if the project has no meaningful session data.
func projectToStatus(p claudeProject) *agent.Status {
	hasTokens := p.LastTotalInputTokens > 0 || p.LastTotalOutputTokens > 0
	hasCost := p.LastCost > 0

	if !hasTokens && !hasCost {
		return nil
	}

	status := &agent.Status{
		CostUSD:        p.LastCost,
		DurationMs:     p.LastDuration,
		APIDurationMs:  p.LastAPIDuration,
		LinesAdded:     p.LastLinesAdded,
		LinesRemoved:   p.LastLinesRemoved,
		AgentSessionID: p.LastSessionID,
		Tokens: agent.TokenUsage{
			InputTokens:      p.LastTotalInputTokens,
			OutputTokens:     p.LastTotalOutputTokens,
			CacheReadTokens:  p.LastTotalCacheReadInputTokens,
			CacheWriteTokens: p.LastTotalCacheCreationInputTokens,
		},
	}

	// Extract model info from the per-model usage breakdown.
	// Use the model with the highest cost as the primary model.
	var bestCost float64
	for modelID, usage := range p.LastModelUsage {
		if usage.CostUSD > bestCost {
			bestCost = usage.CostUSD
			status.Model = agent.ModelInfo{
				ID:          modelID,
				DisplayName: modelDisplayName(modelID),
			}
		}
	}

	return status
}

// modelDisplayName maps Claude model IDs to short display names.
func modelDisplayName(id string) string {
	displayNames := map[string]string{
		"claude-opus-4-6":            "Opus 4.6",
		"claude-sonnet-4-6":          "Sonnet 4.6",
		"claude-haiku-4-5-20251001":  "Haiku 4.5",
		"claude-sonnet-4-5-20250514": "Sonnet 4.5",
		"claude-opus-4-5-20250918":   "Opus 4.5",
	}

	if name, ok := displayNames[id]; ok {
		return name
	}
	return id
}

// claudeConfig is the top-level structure of .claude.json.
type claudeConfig struct {
	Projects map[string]claudeProject `json:"projects"`
}

// claudeProject holds per-project session metrics from .claude.json.
type claudeProject struct {
	LastCost                          float64                     `json:"lastCost"`
	LastAPIDuration                   int64                       `json:"lastAPIDuration"`
	LastDuration                      int64                       `json:"lastDuration"`
	LastLinesAdded                    int                         `json:"lastLinesAdded"`
	LastLinesRemoved                  int                         `json:"lastLinesRemoved"`
	LastTotalInputTokens              int64                       `json:"lastTotalInputTokens"`
	LastTotalOutputTokens             int64                       `json:"lastTotalOutputTokens"`
	LastTotalCacheCreationInputTokens int64                       `json:"lastTotalCacheCreationInputTokens"`
	LastTotalCacheReadInputTokens     int64                       `json:"lastTotalCacheReadInputTokens"`
	LastModelUsage                    map[string]claudeModelUsage `json:"lastModelUsage"`
	LastSessionID                     string                      `json:"lastSessionId"`
}

// claudeModelUsage holds per-model token and cost data.
type claudeModelUsage struct {
	InputTokens      int64   `json:"inputTokens"`
	OutputTokens     int64   `json:"outputTokens"`
	CacheReadTokens  int64   `json:"cacheReadInputTokens"`
	CacheWriteTokens int64   `json:"cacheCreationInputTokens"`
	CostUSD          float64 `json:"costUSD"`
}

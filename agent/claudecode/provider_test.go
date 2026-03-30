package claudecode

import (
	"testing"
)

func TestParseConfig_Empty(t *testing.T) {
	result := ParseConfig(nil)
	if len(result) != 0 {
		t.Errorf("expected empty map for nil input, got %d entries", len(result))
	}

	result = ParseConfig([]byte{})
	if len(result) != 0 {
		t.Errorf("expected empty map for empty input, got %d entries", len(result))
	}
}

func TestParseConfig_InvalidJSON(t *testing.T) {
	result := ParseConfig([]byte("not-json{{{"))
	if len(result) != 0 {
		t.Errorf("expected empty map for invalid JSON, got %d entries", len(result))
	}
}

func TestParseConfig_NoProjects(t *testing.T) {
	result := ParseConfig([]byte(`{"numStartups": 5}`))
	if len(result) != 0 {
		t.Errorf("expected empty map when no projects key, got %d entries", len(result))
	}
}

func TestParseConfig_ProjectWithoutSessionData(t *testing.T) {
	input := []byte(`{
		"projects": {
			"/project": {
				"allowedTools": [],
				"hasTrustDialogAccepted": true
			}
		}
	}`)
	result := ParseConfig(input)
	if len(result) != 0 {
		t.Errorf("expected empty map for project without session data, got %d entries", len(result))
	}
}

func TestParseConfig_SingleProject(t *testing.T) {
	input := []byte(`{
		"projects": {
			"/project/.claude/worktrees/abc-123": {
				"lastCost": 0.0523,
				"lastAPIDuration": 1500,
				"lastDuration": 45000,
				"lastLinesAdded": 156,
				"lastLinesRemoved": 23,
				"lastTotalInputTokens": 15234,
				"lastTotalOutputTokens": 4521,
				"lastTotalCacheCreationInputTokens": 5000,
				"lastTotalCacheReadInputTokens": 2000,
				"lastModelUsage": {
					"claude-opus-4-6": {
						"inputTokens": 15234,
						"outputTokens": 4521,
						"cacheReadInputTokens": 2000,
						"cacheCreationInputTokens": 5000,
						"costUSD": 0.0523
					}
				},
				"lastSessionId": "session-uuid-here"
			}
		}
	}`)

	result := ParseConfig(input)

	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	status, ok := result["/project/.claude/worktrees/abc-123"]
	if !ok {
		t.Fatal("expected entry for /project/.claude/worktrees/abc-123")
	}

	if status.CostUSD != 0.0523 {
		t.Errorf("CostUSD = %f, want 0.0523", status.CostUSD)
	}
	if status.DurationMs != 45000 {
		t.Errorf("DurationMs = %d, want 45000", status.DurationMs)
	}
	if status.APIDurationMs != 1500 {
		t.Errorf("APIDurationMs = %d, want 1500", status.APIDurationMs)
	}
	if status.LinesAdded != 156 {
		t.Errorf("LinesAdded = %d, want 156", status.LinesAdded)
	}
	if status.LinesRemoved != 23 {
		t.Errorf("LinesRemoved = %d, want 23", status.LinesRemoved)
	}
	if status.Tokens.InputTokens != 15234 {
		t.Errorf("InputTokens = %d, want 15234", status.Tokens.InputTokens)
	}
	if status.Tokens.OutputTokens != 4521 {
		t.Errorf("OutputTokens = %d, want 4521", status.Tokens.OutputTokens)
	}
	if status.Tokens.CacheReadTokens != 2000 {
		t.Errorf("CacheReadTokens = %d, want 2000", status.Tokens.CacheReadTokens)
	}
	if status.Tokens.CacheWriteTokens != 5000 {
		t.Errorf("CacheWriteTokens = %d, want 5000", status.Tokens.CacheWriteTokens)
	}
	if status.Model.ID != "claude-opus-4-6" {
		t.Errorf("Model.ID = %q, want %q", status.Model.ID, "claude-opus-4-6")
	}
	if status.Model.DisplayName != "Opus 4.6" {
		t.Errorf("Model.DisplayName = %q, want %q", status.Model.DisplayName, "Opus 4.6")
	}
	if status.AgentSessionID != "session-uuid-here" {
		t.Errorf("AgentSessionID = %q, want %q", status.AgentSessionID, "session-uuid-here")
	}
}

func TestParseConfig_MultipleProjects(t *testing.T) {
	input := []byte(`{
		"projects": {
			"/project/.claude/worktrees/session-a": {
				"lastCost": 0.01,
				"lastTotalInputTokens": 100,
				"lastTotalOutputTokens": 50,
				"lastModelUsage": {
					"claude-sonnet-4-6": {
						"costUSD": 0.01
					}
				}
			},
			"/project/.claude/worktrees/session-b": {
				"lastCost": 0.05,
				"lastTotalInputTokens": 500,
				"lastTotalOutputTokens": 200,
				"lastModelUsage": {
					"claude-opus-4-6": {
						"costUSD": 0.05
					}
				}
			},
			"/project": {
				"allowedTools": []
			}
		}
	}`)

	result := ParseConfig(input)

	if len(result) != 2 {
		t.Fatalf("expected 2 entries (skipping /project with no data), got %d", len(result))
	}

	a := result["/project/.claude/worktrees/session-a"]
	if a == nil {
		t.Fatal("missing session-a")
		return // unreachable but helps staticcheck
	}
	if a.Model.DisplayName != "Sonnet 4.6" {
		t.Errorf("session-a model = %q, want Sonnet 4.6", a.Model.DisplayName)
	}

	b := result["/project/.claude/worktrees/session-b"]
	if b == nil {
		t.Fatal("missing session-b")
		return // unreachable but helps staticcheck
	}
	if b.CostUSD != 0.05 {
		t.Errorf("session-b cost = %f, want 0.05", b.CostUSD)
	}
}

func TestParseConfig_MultipleModels(t *testing.T) {
	input := []byte(`{
		"projects": {
			"/project": {
				"lastCost": 0.10,
				"lastTotalInputTokens": 1000,
				"lastTotalOutputTokens": 500,
				"lastModelUsage": {
					"claude-haiku-4-5-20251001": {
						"costUSD": 0.002
					},
					"claude-opus-4-6": {
						"costUSD": 0.098
					}
				}
			}
		}
	}`)

	result := ParseConfig(input)
	status := result["/project"]
	if status == nil {
		t.Fatal("missing /project entry")
		return // unreachable but helps staticcheck
	}

	// Should pick the highest-cost model as primary.
	if status.Model.ID != "claude-opus-4-6" {
		t.Errorf("Model.ID = %q, want claude-opus-4-6 (highest cost)", status.Model.ID)
	}
}

func TestModelDisplayName(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
		{"unknown-model-id", "unknown-model-id"},
	}

	for _, tt := range tests {
		got := modelDisplayName(tt.id)
		if got != tt.want {
			t.Errorf("modelDisplayName(%q) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

// Verify the Provider satisfies the interface at compile time.
func TestProviderInterface(t *testing.T) {
	p := NewProvider()
	if p.Name() != "claude-code" {
		t.Errorf("Name() = %q, want %q", p.Name(), "claude-code")
	}
	if p.ConfigFilePath() != "/home/dev/.claude.json" {
		t.Errorf("ConfigFilePath() = %q, want /home/dev/.claude.json", p.ConfigFilePath())
	}
}

package codex

import (
	"bufio"
	"os"
	"testing"

	"github.com/thesimonho/warden/agent"
)

func parseFixture(t *testing.T) []agent.ParsedEvent {
	t.Helper()

	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer f.Close()

	parser := NewParser()
	var allEvents []agent.ParsedEvent

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		events := parser.ParseLine(scanner.Bytes())
		allEvents = append(allEvents, events...)
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanning fixture: %v", err)
	}

	return allEvents
}

func TestParseFixture_EventCounts(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	counts := make(map[agent.ParsedEventType]int)
	for _, e := range events {
		counts[e.Type]++
	}

	// 1 session start (session_meta)
	if got := counts[agent.EventSessionStart]; got != 1 {
		t.Errorf("SessionStart events = %d, want 1", got)
	}

	// 3 tool uses (3 exec_command function_calls)
	if got := counts[agent.EventToolUse]; got != 3 {
		t.Errorf("ToolUse events = %d, want 3", got)
	}

	// 1 user prompt
	if got := counts[agent.EventUserPrompt]; got != 1 {
		t.Errorf("UserPrompt events = %d, want 1", got)
	}

	// 3 token updates (one per token_count with info)
	if got := counts[agent.EventTokenUpdate]; got != 3 {
		t.Errorf("TokenUpdate events = %d, want 3", got)
	}

	// 1 turn complete (task_complete)
	if got := counts[agent.EventTurnComplete]; got != 1 {
		t.Errorf("TurnComplete events = %d, want 1", got)
	}
}

func TestParseFixture_ToolNames(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	var toolNames []string
	for _, e := range events {
		if e.Type == agent.EventToolUse {
			toolNames = append(toolNames, e.ToolName)
		}
	}

	// All 3 are exec_command calls
	for i, name := range toolNames {
		if name != "exec_command" {
			t.Errorf("tool[%d] = %q, want %q", i, name, "exec_command")
		}
	}
}

func TestParseFixture_TokensFromLastUpdate(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	var lastTokenEvent agent.ParsedEvent
	for _, e := range events {
		if e.Type == agent.EventTokenUpdate {
			lastTokenEvent = e
		}
	}

	// Last token_count has total_token_usage with 25000 input, 18000 cached
	if lastTokenEvent.Tokens.InputTokens != 25000 {
		t.Errorf("input tokens = %d, want 25000", lastTokenEvent.Tokens.InputTokens)
	}
	if lastTokenEvent.Tokens.CacheReadTokens != 18000 {
		t.Errorf("cache read tokens = %d, want 18000", lastTokenEvent.Tokens.CacheReadTokens)
	}
	// output = 800 regular + 200 reasoning = 1000
	if lastTokenEvent.Tokens.OutputTokens != 1000 {
		t.Errorf("output tokens = %d, want 1000", lastTokenEvent.Tokens.OutputTokens)
	}
}

func TestParseFixture_ModelPopulated(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	for _, e := range events {
		if e.Type == agent.EventTokenUpdate && e.Model == "" {
			t.Error("TokenUpdate event has empty model")
		}
		if e.Type == agent.EventToolUse && e.Model == "" {
			t.Error("ToolUse event has empty model")
		}
	}
}

func TestParseFixture_SessionStart(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	for _, e := range events {
		if e.Type == agent.EventSessionStart {
			if e.SessionID != "test-codex-session-001" {
				t.Errorf("SessionStart ID = %q, want %q", e.SessionID, "test-codex-session-001")
			}
			if e.GitBranch != "main" {
				t.Errorf("SessionStart branch = %q, want %q", e.GitBranch, "main")
			}
			return
		}
	}
	t.Error("no SessionStart event found")
}

func TestParseFixture_EstimatedCost(t *testing.T) {
	t.Parallel()
	events := parseFixture(t)

	var lastCost float64
	for _, e := range events {
		if e.Type == agent.EventTokenUpdate {
			lastCost = e.EstimatedCostUSD
		}
	}

	if lastCost <= 0 {
		t.Errorf("estimated cost = %f, want > 0", lastCost)
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	t.Parallel()
	parser := NewParser()
	events := parser.ParseLine([]byte(`{"timestamp":"2026-01-01T00:00:00Z","type":"future_type","payload":{}}`))
	if len(events) != 0 {
		t.Errorf("unknown type produced %d events, want 0", len(events))
	}
}

func TestParseLine_InvalidJSON(t *testing.T) {
	t.Parallel()
	parser := NewParser()
	events := parser.ParseLine([]byte(`not valid json`))
	if len(events) != 0 {
		t.Errorf("invalid JSON produced %d events, want 0", len(events))
	}
}

func TestSessionDir(t *testing.T) {
	t.Parallel()
	parser := NewParser()
	dir := parser.SessionDir("/home/user", agent.ProjectInfo{
		WorkspaceDir: "/home/dev/project",
		ProjectName:  "project",
	})

	want := "/home/user/.codex/sessions"
	if dir != want {
		t.Errorf("SessionDir = %q, want %q", dir, want)
	}
}

package claudecode

import (
	"bufio"
	"os"
	"testing"

	"github.com/thesimonho/warden/agent"
)

// parseFixtureEvents reads the fixture and collects all parsed events.
func parseFixtureEvents(t *testing.T) []agent.ParsedEvent {
	t.Helper()

	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

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

	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	result, err := agent.ValidateJSONL(NewParser(), f)
	if err != nil {
		t.Fatalf("validating fixture: %v", err)
	}

	// Minimum events any valid Claude session must produce.
	result.Require(agent.EventToolUse, 1)
	result.Require(agent.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("baseline validation failed: %v", err)
	}

	// Exact counts for this fixture.
	if got := result.Counts[agent.EventToolUse]; got != 3 {
		t.Errorf("ToolUse events = %d, want 3", got)
	}
	if got := result.Counts[agent.EventUserPrompt]; got != 1 {
		t.Errorf("UserPrompt events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventTokenUpdate]; got != 4 {
		t.Errorf("TokenUpdate events = %d, want 4", got)
	}
	if got := result.Counts[agent.EventTurnComplete]; got != 1 {
		t.Errorf("TurnComplete events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventTurnDuration]; got != 1 {
		t.Errorf("TurnDuration events = %d, want 1", got)
	}
}

func TestParseFixture_ToolNames(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var toolNames []string
	for _, e := range events {
		if e.Type == agent.EventToolUse {
			toolNames = append(toolNames, e.ToolName)
		}
	}

	want := []string{"Read", "Write", "Bash"}
	if len(toolNames) != len(want) {
		t.Fatalf("tool names = %v, want %v", toolNames, want)
	}
	for i, name := range toolNames {
		if name != want[i] {
			t.Errorf("tool[%d] = %q, want %q", i, name, want[i])
		}
	}
}

func TestParseFixture_TokensAccumulate(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var lastTokenEvent agent.ParsedEvent
	for _, e := range events {
		if e.Type == agent.EventTokenUpdate {
			lastTokenEvent = e
		}
	}

	// Cumulative tokens across all 4 assistant messages:
	// input: 1500+100+80+60 = 1740
	if lastTokenEvent.Tokens.InputTokens != 1740 {
		t.Errorf("cumulative input tokens = %d, want 1740", lastTokenEvent.Tokens.InputTokens)
	}
	// output: 200+150+120+80 = 550
	if lastTokenEvent.Tokens.OutputTokens != 550 {
		t.Errorf("cumulative output tokens = %d, want 550", lastTokenEvent.Tokens.OutputTokens)
	}
	if lastTokenEvent.Tokens.CacheWriteTokens != 5000 {
		t.Errorf("cumulative cache write tokens = %d, want 5000", lastTokenEvent.Tokens.CacheWriteTokens)
	}
	// cache read: 0+4800+5000+5100 = 14900
	if lastTokenEvent.Tokens.CacheReadTokens != 14900 {
		t.Errorf("cumulative cache read tokens = %d, want 14900", lastTokenEvent.Tokens.CacheReadTokens)
	}
}

func TestParseFixture_ModelPopulated(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventTokenUpdate && e.Model == "" {
			t.Error("TokenUpdate event has empty model")
		}
		if e.Type == agent.EventToolUse && e.Model == "" {
			t.Error("ToolUse event has empty model")
		}
	}
}

func TestParseFixture_UserPromptContent(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventUserPrompt {
			if e.Prompt == "" {
				t.Error("UserPrompt event has empty prompt")
			}
			if e.GitBranch != "main" {
				t.Errorf("UserPrompt git branch = %q, want %q", e.GitBranch, "main")
			}
			return
		}
	}
	t.Error("no UserPrompt event found")
}

func TestParseFixture_TurnDuration(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventTurnDuration {
			if e.DurationMs != 7500 {
				t.Errorf("TurnDuration = %d ms, want 7500", e.DurationMs)
			}
			return
		}
	}
	t.Error("no TurnDuration event found")
}

func TestParseFixture_EstimatedCost(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

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

func TestParseFixture_SessionID(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.SessionID != "test-session-001" {
			t.Errorf("event %s has sessionID = %q, want %q", e.Type, e.SessionID, "test-session-001")
		}
	}
}

func TestParseLine_UnknownType(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	events := parser.ParseLine([]byte(`{"type":"some-future-type","uuid":"x","timestamp":"2026-01-01T00:00:00Z"}`))
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

func TestParseLine_EmptyLine(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	events := parser.ParseLine([]byte(``))
	if len(events) != 0 {
		t.Errorf("empty line produced %d events, want 0", len(events))
	}
}

func TestSessionDir(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	dir := parser.SessionDir("/home/user", agent.ProjectInfo{
		WorkspaceDir: "/home/dev/warden",
		ProjectName:  "warden",
	})

	want := "/home/user/.claude/projects/-home-dev-warden"
	if dir != want {
		t.Errorf("SessionDir = %q, want %q", dir, want)
	}
}

// TestValidateLive validates a live JSONL file captured from a real Claude Code
// session. Skipped when VALIDATE_JSONL is not set. Used by CI to verify the
// parser works against the latest CLI output.
func TestValidateLive(t *testing.T) {
	jsonlPath := os.Getenv("VALIDATE_JSONL")
	if jsonlPath == "" {
		t.Skip("VALIDATE_JSONL not set, skipping live validation")
	}

	f, err := os.Open(jsonlPath)
	if err != nil {
		t.Fatalf("opening live JSONL: %v", err)
	}
	defer func() { _ = f.Close() }()

	result, err := agent.ValidateJSONL(NewParser(), f)
	if err != nil {
		t.Fatalf("parsing live JSONL: %v", err)
	}

	result.Require(agent.EventToolUse, 1)
	result.Require(agent.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("live validation failed: %v", err)
	}

	t.Logf("live validation passed: %d total events, counts: %v", result.TotalEvents, result.Counts)
}

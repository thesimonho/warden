package codex

import (
	"bufio"
	"fmt"
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

	// Minimum events any valid Codex session must produce.
	result.Require(agent.EventToolUse, 1)
	result.Require(agent.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("baseline validation failed: %v", err)
	}

	// Exact counts for this fixture.
	if got := result.Counts[agent.EventSessionStart]; got != 1 {
		t.Errorf("SessionStart events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventToolUse]; got != 3 {
		t.Errorf("ToolUse events = %d, want 3", got)
	}
	if got := result.Counts[agent.EventUserPrompt]; got != 1 {
		t.Errorf("UserPrompt events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventTokenUpdate]; got != 3 {
		t.Errorf("TokenUpdate events = %d, want 3", got)
	}
	if got := result.Counts[agent.EventTurnComplete]; got != 1 {
		t.Errorf("TurnComplete events = %d, want 1", got)
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

	for i, name := range toolNames {
		if name != "exec_command" {
			t.Errorf("tool[%d] = %q, want %q", i, name, "exec_command")
		}
	}
}

func TestParseFixture_TokensFromLastUpdate(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

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

func TestParseFixture_SessionStart(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

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

func TestFindSessionFiles(t *testing.T) {
	t.Parallel()

	// Set up a fake ~/.codex structure with shell_snapshots and session files.
	home := t.TempDir()
	projectID := "abc123"

	// Create a shell snapshot for our project.
	snapshotDir := home + "/.codex/shell_snapshots"
	if err := os.MkdirAll(snapshotDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionID := "019d42bf-9d62-73a0-8512-bcfc69932d35"
	snapshotContent := fmt.Sprintf("declare -x WARDEN_PROJECT_ID=%q\n", projectID)
	if err := os.WriteFile(snapshotDir+"/"+sessionID+".12345.sh", []byte(snapshotContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a shell snapshot for a DIFFERENT project.
	otherSessionID := "019d42dd-c765-7dd3-b372-4957b698e30e"
	otherContent := fmt.Sprintf("declare -x WARDEN_PROJECT_ID=%q\n", "other999")
	if err := os.WriteFile(snapshotDir+"/"+otherSessionID+".67890.sh", []byte(otherContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create matching JSONL files in date subdirectories.
	dateDir := home + "/.codex/sessions/2026/03/31"
	if err := os.MkdirAll(dateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	matchFile := dateDir + "/rollout-2026-03-31T07-15-47-" + sessionID + ".jsonl"
	otherFile := dateDir + "/rollout-2026-03-31T07-48-44-" + otherSessionID + ".jsonl"
	if err := os.WriteFile(matchFile, []byte(`{"type":"session_meta"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherFile, []byte(`{"type":"session_meta"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	parser := NewParser()
	files := parser.FindSessionFiles(home, agent.ProjectInfo{
		ProjectID:    projectID,
		WorkspaceDir: "/home/dev/project",
		ProjectName:  "project",
	})

	if len(files) != 1 {
		t.Fatalf("FindSessionFiles returned %d files, want 1", len(files))
	}
	if files[0] != matchFile {
		t.Errorf("FindSessionFiles returned %q, want %q", files[0], matchFile)
	}
}

func TestFindSessionFiles_NoSnapshots(t *testing.T) {
	t.Parallel()
	parser := NewParser()
	files := parser.FindSessionFiles(t.TempDir(), agent.ProjectInfo{ProjectID: "abc123"})
	if len(files) != 0 {
		t.Errorf("expected empty, got %v", files)
	}
}

// TestValidateLive validates a live JSONL file captured from a real Codex
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

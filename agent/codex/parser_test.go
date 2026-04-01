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
	if got := result.Counts[agent.EventToolUse]; got != 7 {
		t.Errorf("ToolUse events = %d, want 7", got)
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
	if got := result.Counts[agent.EventToolUseFailure]; got != 3 {
		t.Errorf("ToolUseFailure events = %d, want 3", got)
	}
	if got := result.Counts[agent.EventStopFailure]; got != 1 {
		t.Errorf("StopFailure events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventPermissionRequest]; got != 1 {
		t.Errorf("PermissionRequest events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventElicitation]; got != 1 {
		t.Errorf("Elicitation events = %d, want 1", got)
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

	want := []string{"exec_command", "exec_command", "exec_command", "exec_command", "exec_command", "search_issues", "patch_apply"}
	if len(toolNames) != len(want) {
		t.Fatalf("tool names = %v, want %v", toolNames, want)
	}
	for i, name := range toolNames {
		if name != want[i] {
			t.Errorf("tool[%d] = %q, want %q", i, name, want[i])
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

func TestParseFixture_ToolUseFailure(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventToolUseFailure {
			if e.ToolName != "exec_command" {
				t.Errorf("ToolUseFailure tool = %q, want %q", e.ToolName, "exec_command")
			}
			if e.ErrorContent == "" {
				t.Error("ToolUseFailure has empty error content")
			}
			return
		}
	}
	t.Error("no ToolUseFailure event found")
}

func TestParseFixture_StopFailure(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventStopFailure {
			if e.ErrorContent == "" {
				t.Error("StopFailure has empty error content")
			}
			return
		}
	}
	t.Error("no StopFailure event found")
}

func TestParseFixture_PermissionRequest(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventPermissionRequest {
			if e.ToolName != "rm -rf /important" {
				t.Errorf("PermissionRequest command = %q, want %q", e.ToolName, "rm -rf /important")
			}
			return
		}
	}
	t.Error("no PermissionRequest event found")
}

func TestParseFixture_Elicitation(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventElicitation {
			if e.ServerName != "github-mcp" {
				t.Errorf("Elicitation server = %q, want %q", e.ServerName, "github-mcp")
			}
			return
		}
	}
	t.Error("no Elicitation event found")
}

func TestIsLikelyError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input string
		want  bool
	}{
		{"error: No such file", true},
		{"Error: connection refused", true},
		{"exit code 1", true},
		{"Exit Code 127", true},
		{"command failed: timeout", true},
		{"permission denied", true},
		{"not found", true},
		{"", false},
		{"success", false},
		{"# Test Project\nA test project.", false},
		{"file created successfully", false},
	}

	for _, tc := range tests {
		got := isLikelyError(tc.input)
		if got != tc.want {
			t.Errorf("isLikelyError(%q) = %v, want %v", tc.input, got, tc.want)
		}
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
		WorkspaceDir: "/home/warden/project",
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
		WorkspaceDir: "/home/warden/project",
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

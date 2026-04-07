package codex

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/event"
)

// parseFixtureEvents reads the fixture and collects all parsed events.
func parseFixtureEvents(t *testing.T) []agent.ParsedEvent {
	t.Helper()

	f, err := os.Open("testdata/session.jsonl")
	if err != nil {
		t.Fatalf("opening fixture: %v", err)
	}
	defer func() { _ = f.Close() }()

	events, err := agent.ParseAllEvents(NewParser(), f)
	if err != nil {
		t.Fatalf("parsing fixture: %v", err)
	}
	return events
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
	result.Require(event.EventToolUse, 1)
	result.Require(event.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("baseline validation failed: %v", err)
	}

	// Exact counts for this fixture.
	if got := result.Counts[event.EventSessionStart]; got != 1 {
		t.Errorf("SessionStart events = %d, want 1", got)
	}
	if got := result.Counts[event.EventToolUse]; got != 9 {
		t.Errorf("ToolUse events = %d, want 9", got)
	}
	if got := result.Counts[event.EventUserPrompt]; got != 5 {
		t.Errorf("UserPrompt events = %d, want 5", got)
	}
	if got := result.Counts[event.EventTokenUpdate]; got != 3 {
		t.Errorf("TokenUpdate events = %d, want 3", got)
	}
	if got := result.Counts[event.EventTurnComplete]; got != 1 {
		t.Errorf("TurnComplete events = %d, want 1", got)
	}
	if got := result.Counts[event.EventToolUseFailure]; got != 3 {
		t.Errorf("ToolUseFailure events = %d, want 3", got)
	}
	if got := result.Counts[event.EventStopFailure]; got != 2 {
		t.Errorf("StopFailure events = %d, want 2", got)
	}
	if got := result.Counts[event.EventContextCompact]; got != 3 {
		t.Errorf("ContextCompact events = %d, want 3", got)
	}
}

func TestParseFixture_ToolNames(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var toolNames []string
	for _, e := range events {
		if e.Type == event.EventToolUse {
			toolNames = append(toolNames, e.ToolName)
		}
	}

	want := []string{
		"exec_command", "exec_command", "exec_command", "exec_command",
		"shell", "web_search", "lint_file", "image_generation", "tool_search",
	}
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
		if e.Type == event.EventTokenUpdate {
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
		if e.Type == event.EventTokenUpdate && e.Model == "" {
			t.Error("TokenUpdate event has empty model")
		}
		if e.Type == event.EventToolUse && e.Model == "" {
			t.Error("ToolUse event has empty model")
		}
	}
}

func TestParseFixture_SessionStart(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == event.EventSessionStart {
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
		if e.Type == event.EventTokenUpdate {
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

	var failures []string
	for _, e := range events {
		if e.Type == event.EventToolUseFailure {
			failures = append(failures, e.ErrorContent)
		}
	}
	// function_call_output heuristic + exec_command_end exit code 1 + mcp_tool_call_end error
	if len(failures) != 3 {
		t.Fatalf("ToolUseFailure events = %d (%v), want 3", len(failures), failures)
	}
}

func TestParseFixture_StopFailure(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == event.EventStopFailure {
			if e.ErrorContent == "" {
				t.Error("StopFailure has empty error content")
			}
			return
		}
	}
	t.Error("no StopFailure event found")
}

func TestParseFixture_ContextCompact(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var triggers []string
	for _, e := range events {
		if e.Type == event.EventContextCompact {
			triggers = append(triggers, e.CompactTrigger)
		}
	}
	// Three sources: context_compacted, thread_rolled_back ("rollback"), compacted ("compacted").
	if len(triggers) != 3 {
		t.Fatalf("ContextCompact events = %d (%v), want 3", len(triggers), triggers)
	}
}

func TestParseFixture_TurnAborted(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == event.EventStopFailure && e.ErrorContent == "turn aborted: interrupted" {
			return
		}
	}
	t.Error("no StopFailure event for turn_aborted found")
}

func TestParseFixture_LocalShellCall(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == event.EventToolUse && e.ToolName == "shell" {
			if e.ToolInput != "ls -la /tmp" {
				t.Errorf("shell ToolInput = %q, want %q", e.ToolInput, "ls -la /tmp")
			}
			return
		}
	}
	t.Error("no ToolUse event for local_shell_call found")
}

func TestParseFixture_CustomToolCall(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == event.EventToolUse && e.ToolName == "lint_file" {
			if e.ToolInput != "src/main.rs" {
				t.Errorf("custom tool ToolInput = %q, want %q", e.ToolInput, "src/main.rs")
			}
			return
		}
	}
	t.Error("no ToolUse event for custom_tool_call found")
}

func TestParseFixture_UserShellCommand(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var bashCommands, bashOutputs []agent.ParsedEvent
	for _, e := range events {
		if e.Type != event.EventUserPrompt {
			continue
		}
		switch e.PromptSource {
		case agent.PromptSourceBash:
			bashCommands = append(bashCommands, e)
		case agent.PromptSourceBashOutput:
			bashOutputs = append(bashOutputs, e)
		}
	}

	// Two user shell commands in fixture: curl and ls.
	if len(bashCommands) != 2 {
		t.Fatalf("bash command events = %d, want 2", len(bashCommands))
	}
	// Codex wraps in /bin/bash -lc — extractUserCommand strips the wrapper.
	if bashCommands[0].Prompt != "$ curl example.com" {
		t.Errorf("bash command[0] = %q, want %q", bashCommands[0].Prompt, "$ curl example.com")
	}
	if bashCommands[1].Prompt != "$ ls -la" {
		t.Errorf("bash command[1] = %q, want %q", bashCommands[1].Prompt, "$ ls -la")
	}

	// Both have non-empty output (curl has stderr, ls has stdout).
	if len(bashOutputs) != 2 {
		t.Fatalf("bash output events = %d, want 2", len(bashOutputs))
	}
	if bashOutputs[0].Prompt != "% Total    % Received\ncurl: (7) Failed to connect" {
		t.Errorf("bash output[0] = %q", bashOutputs[0].Prompt)
	}
}

func TestExtractUserCommand(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		command []string
		want    string
	}{
		{"bash wrapper", []string{"/bin/bash", "-lc", "curl example.com"}, "curl example.com"},
		{"bare command", []string{"ls", "-la"}, "ls -la"},
		{"empty", nil, ""},
		{"just bash no -lc", []string{"/bin/bash", "script.sh"}, "/bin/bash script.sh"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := extractUserCommand(tc.command)
			if got != tc.want {
				t.Errorf("extractUserCommand(%v) = %q, want %q", tc.command, got, tc.want)
			}
		})
	}
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

// TestSessionDetectionGlob verifies that the glob pattern used in
// create-terminal.sh matches files at the actual Codex session path depth
// (year/month/day/file.jsonl — 4 levels under ~/.codex/sessions/).
func TestSessionDetectionGlob(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	sessionsDir := filepath.Join(home, ".codex", "sessions", "2026", "04", "02")
	if err := os.MkdirAll(sessionsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(sessionsDir, "rollout-2026-04-02T08-40-23-abc123.jsonl")
	if err := os.WriteFile(sessionFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// This glob must match the pattern in create-terminal.sh:
	//   ls ~/.codex/sessions/*/*/*/*.jsonl
	glob := filepath.Join(home, ".codex", "sessions", "*", "*", "*", "*.jsonl")
	matches, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match for session glob, got %d (glob: %s)", len(matches), glob)
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

	result.Require(event.EventToolUse, 1)
	result.Require(event.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("live validation failed: %v", err)
	}

	t.Logf("live validation passed: %d total events, counts: %v", result.TotalEvents, result.Counts)
}

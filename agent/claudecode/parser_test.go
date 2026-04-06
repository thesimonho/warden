package claudecode

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/thesimonho/warden/agent"
)

// filterOut returns events with the given type removed.
func filterOut(events []agent.ParsedEvent, eventType agent.ParsedEventType) []agent.ParsedEvent {
	var filtered []agent.ParsedEvent
	for _, e := range events {
		if e.Type != eventType {
			filtered = append(filtered, e)
		}
	}
	return filtered
}

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

	// Minimum events any valid Claude session must produce.
	result.Require(agent.EventSessionStart, 1)
	result.Require(agent.EventToolUse, 1)
	result.Require(agent.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("baseline validation failed: %v", err)
	}

	// Exact counts for this fixture.
	if got := result.Counts[agent.EventSessionStart]; got != 1 {
		t.Errorf("SessionStart events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventToolUse]; got != 4 {
		t.Errorf("ToolUse events = %d, want 4", got)
	}
	if got := result.Counts[agent.EventUserPrompt]; got != 2 {
		t.Errorf("UserPrompt events = %d, want 2", got)
	}
	if got := result.Counts[agent.EventTokenUpdate]; got != 5 {
		t.Errorf("TokenUpdate events = %d, want 5", got)
	}
	if got := result.Counts[agent.EventTurnComplete]; got != 1 {
		t.Errorf("TurnComplete events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventTurnDuration]; got != 1 {
		t.Errorf("TurnDuration events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventToolUseFailure]; got != 1 {
		t.Errorf("ToolUseFailure events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventStopFailure]; got != 1 {
		t.Errorf("StopFailure events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventSubagentStop]; got != 1 {
		t.Errorf("SubagentStop events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventApiMetrics]; got != 1 {
		t.Errorf("ApiMetrics events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventPermissionGrant]; got != 1 {
		t.Errorf("PermissionGrant events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventContextCompact]; got != 1 {
		t.Errorf("ContextCompact events = %d, want 1", got)
	}
	if got := result.Counts[agent.EventSystemInfo]; got != 2 {
		t.Errorf("SystemInfo events = %d, want 2", got)
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

	want := []string{"Read", "Write", "Bash", "Read"}
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

	// Cumulative tokens across all 5 assistant messages:
	// input: 1500+100+80+60+40 = 1780
	if lastTokenEvent.Tokens.InputTokens != 1780 {
		t.Errorf("cumulative input tokens = %d, want 1780", lastTokenEvent.Tokens.InputTokens)
	}
	// output: 200+150+120+80+30 = 580
	if lastTokenEvent.Tokens.OutputTokens != 580 {
		t.Errorf("cumulative output tokens = %d, want 580", lastTokenEvent.Tokens.OutputTokens)
	}
	if lastTokenEvent.Tokens.CacheWriteTokens != 5000 {
		t.Errorf("cumulative cache write tokens = %d, want 5000", lastTokenEvent.Tokens.CacheWriteTokens)
	}
	// cache read: 0+4800+5000+5100+5200 = 20100
	if lastTokenEvent.Tokens.CacheReadTokens != 20100 {
		t.Errorf("cumulative cache read tokens = %d, want 20100", lastTokenEvent.Tokens.CacheReadTokens)
	}
}

func TestParseFixture_ModelPopulated(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventTokenUpdate {
			if e.Model == "" {
				t.Error("TokenUpdate event has empty model")
			}
			if e.Model != "claude-sonnet-4-6" {
				t.Errorf("TokenUpdate model = %q, want %q", e.Model, "claude-sonnet-4-6")
			}
		}
		if e.Type == agent.EventToolUse {
			if e.Model == "" {
				t.Error("ToolUse event has empty model")
			}
			if e.Model != "claude-sonnet-4-6" {
				t.Errorf("ToolUse model = %q, want %q", e.Model, "claude-sonnet-4-6")
			}
		}
	}
}

func TestParseFixture_UserPromptContent(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var prompts []agent.ParsedEvent
	for _, e := range events {
		if e.Type == agent.EventUserPrompt {
			if e.Prompt == "" {
				t.Error("UserPrompt event has empty prompt")
			}
			prompts = append(prompts, e)
		}
	}
	if len(prompts) != 2 {
		t.Fatalf("expected 2 UserPrompt events, got %d", len(prompts))
	}

	// First prompt: direct user message with git branch.
	wantFirst := "Read the file README.md, then create a file called /tmp/warden-test.txt with hello, then delete it"
	if prompts[0].Prompt != wantFirst {
		t.Errorf("first prompt = %q, want %q", prompts[0].Prompt, wantFirst)
	}
	if prompts[0].GitBranch != "main" {
		t.Errorf("first prompt git branch = %q, want %q", prompts[0].GitBranch, "main")
	}

	// Second prompt: enqueued via queue-operation (no git branch).
	if prompts[1].Prompt != "also check the tests" {
		t.Errorf("second prompt = %q, want %q", prompts[1].Prompt, "also check the tests")
	}
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

func TestParseFixture_SessionStartIsFirstEvent(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	if len(events) == 0 {
		t.Fatal("no events parsed")
	}
	if events[0].Type != agent.EventSessionStart {
		t.Errorf("first event type = %s, want %s", events[0].Type, agent.EventSessionStart)
	}
	if events[0].SessionID != "test-session-001" {
		t.Errorf("SessionStart sessionID = %q, want %q", events[0].SessionID, "test-session-001")
	}
}

func TestParseLine_SessionStartEmittedOncePerSessionID(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// First line with session ID — should produce SessionStart + UserPrompt.
	events := parser.ParseLine([]byte(`{"type":"user","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","cwd":"/home/warden/project","message":{"content":"hello","role":"user"}}`))
	var sessionStarts int
	for _, e := range events {
		if e.Type == agent.EventSessionStart {
			sessionStarts++
		}
	}
	if sessionStarts != 1 {
		t.Errorf("first line: SessionStart count = %d, want 1", sessionStarts)
	}

	// Second line with same session ID — no SessionStart.
	events = parser.ParseLine([]byte(`{"type":"user","sessionId":"s1","timestamp":"2026-01-01T00:00:01Z","cwd":"/home/warden/project","message":{"content":"world","role":"user"}}`))
	for _, e := range events {
		if e.Type == agent.EventSessionStart {
			t.Error("second line with same sessionId should not emit SessionStart")
		}
	}
}

func TestParseLine_SessionStartOnSessionIDChange(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// First session.
	parser.ParseLine([]byte(`{"type":"user","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","cwd":"/home/warden/project","message":{"content":"hello","role":"user"}}`))

	// New session ID — should emit SessionStart again.
	events := parser.ParseLine([]byte(`{"type":"user","sessionId":"s2","timestamp":"2026-01-01T00:01:00Z","cwd":"/home/warden/project","message":{"content":"resumed","role":"user"}}`))
	var found bool
	for _, e := range events {
		if e.Type == agent.EventSessionStart {
			found = true
			if e.SessionID != "s2" {
				t.Errorf("SessionStart sessionID = %q, want %q", e.SessionID, "s2")
			}
		}
	}
	if !found {
		t.Error("new sessionId should emit SessionStart")
	}
}

func TestParseLine_SessionStartResetsTokenAccumulation(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// First session — accumulate some tokens.
	parser.ParseLine([]byte(`{"type":"assistant","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","cwd":"/home/warden/project","message":{"model":"claude-sonnet-4-6","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":1000,"output_tokens":200}}}`))

	// New session — tokens should reset.
	events := parser.ParseLine([]byte(`{"type":"assistant","sessionId":"s2","timestamp":"2026-01-01T00:01:00Z","cwd":"/home/warden/project","message":{"model":"claude-sonnet-4-6","role":"assistant","content":[{"type":"text","text":"hi"}],"usage":{"input_tokens":500,"output_tokens":100}}}`))

	for _, e := range events {
		if e.Type == agent.EventTokenUpdate {
			if e.Tokens.InputTokens != 500 {
				t.Errorf("cumulative input tokens = %d, want 500 (reset for new session)", e.Tokens.InputTokens)
			}
			if e.Tokens.OutputTokens != 100 {
				t.Errorf("cumulative output tokens = %d, want 100 (reset for new session)", e.Tokens.OutputTokens)
			}
			return
		}
	}
	t.Error("no TokenUpdate event found")
}

func TestParseFixture_ToolUseFailure(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventToolUseFailure {
			if e.ToolName != "Read" {
				t.Errorf("ToolUseFailure tool = %q, want %q", e.ToolName, "Read")
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
			if e.SessionID != "test-session-001" {
				t.Errorf("StopFailure sessionID = %q, want %q", e.SessionID, "test-session-001")
			}
			return
		}
	}
	t.Error("no StopFailure event found")
}

func TestParseFixture_SubagentStop(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventSubagentStop {
			if e.Content != "Killed 2 subagents" {
				t.Errorf("SubagentStop content = %q, want %q", e.Content, "Killed 2 subagents")
			}
			return
		}
	}
	t.Error("no SubagentStop event found")
}

func TestParseFixture_ApiMetrics(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventApiMetrics {
			if e.TTFTMs != 1234.5 {
				t.Errorf("TTFTMs = %f, want 1234.5", e.TTFTMs)
			}
			if e.OutputTokensPerSec != 56.7 {
				t.Errorf("OutputTokensPerSec = %f, want 56.7", e.OutputTokensPerSec)
			}
			return
		}
	}
	t.Error("no ApiMetrics event found")
}

func TestParseFixture_PermissionGrant(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventPermissionGrant {
			if len(e.Commands) != 2 {
				t.Fatalf("PermissionGrant commands = %v, want 2 items", e.Commands)
			}
			if e.Commands[0] != "git push" {
				t.Errorf("PermissionGrant commands[0] = %q, want %q", e.Commands[0], "git push")
			}
			return
		}
	}
	t.Error("no PermissionGrant event found")
}

func TestParseFixture_ContextCompact(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	for _, e := range events {
		if e.Type == agent.EventContextCompact {
			if e.CompactTrigger != "auto" {
				t.Errorf("CompactTrigger = %q, want %q", e.CompactTrigger, "auto")
			}
			if e.PreCompactTokens != 150000 {
				t.Errorf("PreCompactTokens = %d, want 150000", e.PreCompactTokens)
			}
			return
		}
	}
	t.Error("no ContextCompact event found")
}

func TestParseFixture_SystemInfo(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	var subtypes []string
	for _, e := range events {
		if e.Type == agent.EventSystemInfo {
			subtypes = append(subtypes, e.Subtype)
			if e.Content == "" {
				t.Errorf("SystemInfo %q has empty content", e.Subtype)
			}
		}
	}
	if len(subtypes) != 2 {
		t.Fatalf("SystemInfo subtypes = %v, want [informational memory_saved]", subtypes)
	}
}

func TestParseFixture_WorktreeIDFromMainCWD(t *testing.T) {
	t.Parallel()
	events := parseFixtureEvents(t)

	// All fixture entries have cwd="/home/warden/project" (main workspace).
	// WorktreeIDFromCWD returns "main" for non-worktree paths.
	for _, e := range events {
		if e.WorktreeID != "main" {
			t.Errorf("event %s has WorktreeID = %q, want %q", e.Type, e.WorktreeID, "main")
		}
	}
}

func TestParseLine_WorktreeIDFromClaude(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// User prompt from a Claude Code worktree.
	events := parser.ParseLine([]byte(`{"type":"user","cwd":"/home/warden/project/.claude/worktrees/fix-auth","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","message":{"content":"hello","role":"user"}}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].WorktreeID != "fix-auth" {
		t.Errorf("WorktreeID = %q, want %q", events[0].WorktreeID, "fix-auth")
	}
}

func TestParseLine_WorktreeIDWithUnderscore(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// Worktree name with underscores — should be preserved exactly.
	events := parser.ParseLine([]byte(`{"type":"user","cwd":"/home/warden/project/.claude/worktrees/tools_again","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","message":{"content":"hello","role":"user"}}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].WorktreeID != "tools_again" {
		t.Errorf("WorktreeID = %q, want %q", events[0].WorktreeID, "tools_again")
	}
}

func TestParseLine_WorktreeIDFromSubdirectory(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// CWD is a subdirectory inside the worktree — should still extract the worktree ID.
	events := parser.ParseLine([]byte(`{"type":"user","cwd":"/home/warden/project/.claude/worktrees/my-branch/src/lib","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","message":{"content":"hello","role":"user"}}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].WorktreeID != "my-branch" {
		t.Errorf("WorktreeID = %q, want %q", events[0].WorktreeID, "my-branch")
	}
}

func TestParseLine_WorktreeIDNoCWD(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// No CWD field — WorktreeID should remain empty (callback defaults to "main").
	events := parser.ParseLine([]byte(`{"type":"system","subtype":"turn_duration","durationMs":5000,"sessionId":"s1","timestamp":"2026-01-01T00:00:00Z"}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].WorktreeID != "" {
		t.Errorf("WorktreeID = %q, want empty (no CWD)", events[0].WorktreeID)
	}
}

func TestParseLine_WorktreeIDOnAllEventTypes(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	worktreeCWD := "/home/warden/project/.claude/worktrees/feature-x"

	// Assistant entry — should set WorktreeID on token_update and tool_use events.
	assistantLine := []byte(`{"type":"assistant","cwd":"` + worktreeCWD + `","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","message":{"model":"claude-sonnet-4-6","role":"assistant","stop_reason":"end_turn","content":[{"type":"text","text":"done"}],"usage":{"input_tokens":100,"output_tokens":50}}}`)
	events := parser.ParseLine(assistantLine)
	for _, e := range events {
		if e.WorktreeID != "feature-x" {
			t.Errorf("assistant event %s has WorktreeID = %q, want %q", e.Type, e.WorktreeID, "feature-x")
		}
	}

	// System entry — should set WorktreeID on system events.
	systemLine := []byte(`{"type":"system","subtype":"api_error","content":"rate limited","cwd":"` + worktreeCWD + `","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z"}`)
	events = parser.ParseLine(systemLine)
	for _, e := range events {
		if e.WorktreeID != "feature-x" {
			t.Errorf("system event %s has WorktreeID = %q, want %q", e.Type, e.WorktreeID, "feature-x")
		}
	}
}

func TestParseLine_WorktreeIDFromWardenManaged(t *testing.T) {
	t.Parallel()

	parser := NewParser()

	// Warden-managed worktrees use .warden/worktrees/ prefix (Codex pattern).
	events := parser.ParseLine([]byte(`{"type":"user","cwd":"/home/warden/project/.warden/worktrees/my-fix","sessionId":"s1","timestamp":"2026-01-01T00:00:00Z","message":{"content":"hello","role":"user"}}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].WorktreeID != "my-fix" {
		t.Errorf("WorktreeID = %q, want %q", events[0].WorktreeID, "my-fix")
	}
}

func TestParseLine_QueueOperationEnqueue(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	events := parser.ParseLine([]byte(`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-01-01T00:00:00Z","sessionId":"s1","content":"fix the tests","cwd":"/home/warden/project"}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Type != agent.EventUserPrompt {
		t.Errorf("event type = %s, want %s", events[0].Type, agent.EventUserPrompt)
	}
	if events[0].Prompt != "fix the tests" {
		t.Errorf("prompt = %q, want %q", events[0].Prompt, "fix the tests")
	}
	if events[0].WorktreeID != "main" {
		t.Errorf("WorktreeID = %q, want %q", events[0].WorktreeID, "main")
	}
}

func TestParseLine_QueueOperationRemove(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	events := parser.ParseLine([]byte(`{"type":"queue-operation","operation":"remove","timestamp":"2026-01-01T00:00:00Z","sessionId":"s1"}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 0 {
		t.Errorf("remove operation produced %d events, want 0", len(events))
	}
}

func TestParseLine_QueueOperationEmptyContent(t *testing.T) {
	t.Parallel()

	parser := NewParser()
	events := parser.ParseLine([]byte(`{"type":"queue-operation","operation":"enqueue","timestamp":"2026-01-01T00:00:00Z","sessionId":"s1","content":""}`))
	events = filterOut(events, agent.EventSessionStart)
	if len(events) != 0 {
		t.Errorf("empty content enqueue produced %d events, want 0", len(events))
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
		WorkspaceDir: "/home/warden/my-project",
		ProjectName:  "my-project",
	})

	want := "/home/user/.claude/projects/-home-warden-my-project"
	if dir != want {
		t.Errorf("SessionDir = %q, want %q", dir, want)
	}
}

// TestSessionDetectionGlob verifies that the glob pattern used in
// create-terminal.sh matches files at the actual Claude Code session path
// depth (project-dir/*.jsonl — 2 levels under ~/.claude/projects/).
func TestSessionDetectionGlob(t *testing.T) {
	t.Parallel()

	home := t.TempDir()
	projectDir := filepath.Join(home, ".claude", "projects", "-home-warden-my-project")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	sessionFile := filepath.Join(projectDir, "abc123-session.jsonl")
	if err := os.WriteFile(sessionFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	// This glob must match the pattern in create-terminal.sh:
	//   ls ~/.claude/projects/*/*.jsonl
	glob := filepath.Join(home, ".claude", "projects", "*", "*.jsonl")
	matches, err := filepath.Glob(glob)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Errorf("expected 1 match for session glob, got %d (glob: %s)", len(matches), glob)
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

	result.Require(agent.EventSessionStart, 1)
	result.Require(agent.EventToolUse, 1)
	result.Require(agent.EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Fatalf("live validation failed: %v", err)
	}

	t.Logf("live validation passed: %d total events, counts: %v", result.TotalEvents, result.Counts)
}

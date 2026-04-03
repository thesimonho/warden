package agent

import (
	"strings"
	"testing"
)

type stubParser struct{}

func (s *stubParser) ParseLine(line []byte) []ParsedEvent {
	text := string(line)
	switch {
	case strings.Contains(text, "tool"):
		return []ParsedEvent{{Type: EventToolUse}}
	case strings.Contains(text, "token"):
		return []ParsedEvent{{Type: EventTokenUpdate}}
	default:
		return nil
	}
}

func (s *stubParser) SessionDir(string, ProjectInfo) string         { return "" }
func (s *stubParser) FindSessionFiles(string, ProjectInfo) []string { return nil }

func TestValidateJSONL_CountsEvents(t *testing.T) {
	t.Parallel()

	input := "tool_use line\ntoken line\ntool_use line\nunknown line\n"
	result, err := ValidateJSONL(&stubParser{}, strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.TotalEvents != 3 {
		t.Errorf("TotalEvents = %d, want 3", result.TotalEvents)
	}
	if result.Counts[EventToolUse] != 2 {
		t.Errorf("ToolUse = %d, want 2", result.Counts[EventToolUse])
	}
	if result.Counts[EventTokenUpdate] != 1 {
		t.Errorf("TokenUpdate = %d, want 1", result.Counts[EventTokenUpdate])
	}
}

func TestValidationResult_Require(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{
		TotalEvents: 5,
		Counts: map[ParsedEventType]int{
			EventToolUse:     3,
			EventTokenUpdate: 2,
		},
	}

	result.Require(EventToolUse, 1)
	result.Require(EventTokenUpdate, 1)
	if err := result.Check(); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestValidationResult_RequireFails(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{
		TotalEvents: 1,
		Counts: map[ParsedEventType]int{
			EventToolUse: 1,
		},
	}

	result.Require(EventToolUse, 1)
	result.Require(EventTokenUpdate, 1) // fails: 0 < 1
	if err := result.Check(); err == nil {
		t.Error("expected error, got nil")
	}
}

func TestValidationResult_CheckNoEvents(t *testing.T) {
	t.Parallel()

	result := &ValidationResult{
		TotalEvents: 0,
		Counts:      map[ParsedEventType]int{},
	}

	if err := result.Check(); err == nil {
		t.Error("expected error for empty events, got nil")
	}
}

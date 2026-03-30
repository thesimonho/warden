package service

import (
	"testing"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/eventbus"
)

// TestEventTypeMapping verifies that ParsedEventType string values match
// the corresponding ContainerEventType constants. These are kept in separate
// packages to avoid an import cycle, so this test catches drift.
func TestEventTypeMapping(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		parsed    agent.ParsedEventType
		container eventbus.ContainerEventType
	}{
		{agent.EventSessionStart, eventbus.EventSessionStart},
		{agent.EventSessionEnd, eventbus.EventSessionEnd},
		{agent.EventToolUse, eventbus.EventToolUse},
		{agent.EventUserPrompt, eventbus.EventUserPrompt},
	}

	for _, p := range pairs {
		if string(p.parsed) != string(p.container) {
			t.Errorf("event type mismatch: agent.%s=%q != eventbus.%s=%q",
				p.parsed, p.parsed, p.container, p.container)
		}
	}
}

func TestSessionEventToContainerEvent_NilForUnmappedTypes(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "test", ContainerName: "test", WorktreeID: "main"}
	unmappedTypes := []agent.ParsedEventType{
		agent.EventTurnComplete,
		agent.EventTurnDuration,
	}

	for _, typ := range unmappedTypes {
		event := agent.ParsedEvent{Type: typ, SessionID: "s1", Timestamp: "2026-01-01T00:00:00Z"}
		result := SessionEventToContainerEvent(event, ctx)
		if result != nil {
			t.Errorf("expected nil for %s, got %+v", typ, result)
		}
	}
}

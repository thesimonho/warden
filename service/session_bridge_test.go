package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/event"
)

// TestEventTypeMapping verifies that ParsedEventType string values match
// the corresponding ContainerEventType constants. These are kept in separate
// packages to avoid an import cycle, so this test catches drift.
func TestEventTypeMapping(t *testing.T) {
	t.Parallel()

	pairs := []struct {
		parsed    agent.ParsedEventType
		container event.ContainerEventType
	}{
		{event.EventSessionStart, event.EventSessionStart},
		{event.EventSessionEnd, event.EventSessionEnd},
		{event.EventToolUse, event.EventToolUse},
		{event.EventUserPrompt, event.EventUserPrompt},
	}

	for _, p := range pairs {
		if string(p.parsed) != string(p.container) {
			t.Errorf("event type mismatch: agent.%s=%q != event.%s=%q",
				p.parsed, p.parsed, p.container, p.container)
		}
	}
}

func TestSessionEventToContainerEvent_NilForUnknownType(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "test", ContainerName: "test", WorktreeID: "main"}

	// A synthetic type that doesn't map to any container event should return nil.
	evt := agent.ParsedEvent{Type: "unknown_future_event", SessionID: "s1", Timestamp: "2026-01-01T00:00:00Z"}
	result := SessionEventToContainerEvent(evt, ctx)
	if result != nil {
		t.Errorf("expected nil for unknown event type, got %+v", result)
	}
}

func TestSessionEventToContainerEvent_ToolUsePayload(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "proj-1", ContainerName: "warden-test", WorktreeID: "feat-x"}
	evt := agent.ParsedEvent{
		Type:      event.EventToolUse,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00.000Z",
		ToolName:  "Bash",
		ToolInput: "ls -la",
	}

	result := SessionEventToContainerEvent(evt, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != event.EventToolUse {
		t.Errorf("Type = %q, want %q", result.Type, event.EventToolUse)
	}

	var data event.ToolUseData
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal ToolUseData: %v", err)
	}
	if data.ToolName != "Bash" {
		t.Errorf("ToolName = %q, want %q", data.ToolName, "Bash")
	}
	if data.ToolInput != "ls -la" {
		t.Errorf("ToolInput = %q, want %q", data.ToolInput, "ls -la")
	}
}

func TestSessionEventToContainerEvent_UserPromptPayload(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "proj-1", ContainerName: "warden-test", WorktreeID: "main"}
	evt := agent.ParsedEvent{
		Type:      event.EventUserPrompt,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
		Prompt:    "fix the bug in main.go",
	}

	result := SessionEventToContainerEvent(evt, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != event.EventUserPrompt {
		t.Errorf("Type = %q, want %q", result.Type, event.EventUserPrompt)
	}

	var data map[string]string
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal prompt data: %v", err)
	}
	if data["prompt"] != "fix the bug in main.go" {
		t.Errorf("prompt = %q, want %q", data["prompt"], "fix the bug in main.go")
	}
}

func TestSessionEventToContainerEvent_TokenUpdatePayload(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "proj-1", ContainerName: "warden-test", WorktreeID: "main"}
	evt := agent.ParsedEvent{
		Type:             event.EventTokenUpdate,
		SessionID:        "sess-123",
		Timestamp:        "2026-03-30T10:00:00Z",
		EstimatedCostUSD: 0.0042,
	}

	result := SessionEventToContainerEvent(evt, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// TokenUpdate maps to EventCostUpdate for the cost persistence pipeline.
	if result.Type != event.EventCostUpdate {
		t.Errorf("Type = %q, want %q (TokenUpdate maps to CostUpdate)", result.Type, event.EventCostUpdate)
	}

	var data event.CostData
	if err := json.Unmarshal(result.Data, &data); err != nil {
		t.Fatalf("failed to unmarshal CostData: %v", err)
	}
	if math.Abs(data.TotalCost-0.0042) > 1e-9 {
		t.Errorf("TotalCost = %f, want %f", data.TotalCost, 0.0042)
	}
	if !data.IsEstimated {
		t.Error("IsEstimated = false, want true")
	}
	if data.SessionID != "sess-123" {
		t.Errorf("SessionID = %q, want %q", data.SessionID, "sess-123")
	}
}

func TestSessionEventToContainerEvent_SessionStartNoData(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "proj-1", ContainerName: "warden-test", WorktreeID: "main"}
	evt := agent.ParsedEvent{
		Type:      event.EventSessionStart,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
	}

	result := SessionEventToContainerEvent(evt, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != event.EventSessionStart {
		t.Errorf("Type = %q, want %q", result.Type, event.EventSessionStart)
	}
	if result.Data != nil {
		t.Errorf("Data = %s, want nil (session_start has no payload)", string(result.Data))
	}
}

func TestSessionEventToContainerEvent_ContextFields(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{
		ProjectID:     "abc123def456",
		ContainerName: "warden-my-project",
		WorktreeID:    "feat-login",
	}
	evt := agent.ParsedEvent{
		Type:      event.EventToolUse,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
		ToolName:  "Read",
	}

	result := SessionEventToContainerEvent(evt, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.ProjectID != ctx.ProjectID {
		t.Errorf("ProjectID = %q, want %q", result.ProjectID, ctx.ProjectID)
	}
	if result.ContainerName != ctx.ContainerName {
		t.Errorf("ContainerName = %q, want %q", result.ContainerName, ctx.ContainerName)
	}
	if result.WorktreeID != ctx.WorktreeID {
		t.Errorf("WorktreeID = %q, want %q", result.WorktreeID, ctx.WorktreeID)
	}
}

func TestSessionEventToContainerEvent_TimestampParsing(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "test", ContainerName: "test", WorktreeID: "main"}

	// RFC3339Nano format.
	evt := agent.ParsedEvent{
		Type:      event.EventSessionStart,
		Timestamp: "2026-03-30T10:15:30.123456789Z",
	}
	result := SessionEventToContainerEvent(evt, ctx)
	if result.Timestamp.Year() != 2026 || result.Timestamp.Month() != 3 || result.Timestamp.Day() != 30 {
		t.Errorf("RFC3339Nano: timestamp = %v, want 2026-03-30", result.Timestamp)
	}

	// Millisecond format (fallback).
	evt.Timestamp = "2026-03-30T10:15:30.123Z"
	result = SessionEventToContainerEvent(evt, ctx)
	if result.Timestamp.Year() != 2026 {
		t.Errorf("millisecond format: timestamp = %v, want year 2026", result.Timestamp)
	}

	// Invalid format falls back to approximately now.
	evt.Timestamp = "not-a-timestamp"
	result = SessionEventToContainerEvent(evt, ctx)
	if time.Since(result.Timestamp) > 5*time.Second {
		t.Errorf("invalid timestamp: expected ~now, got %v (diff %v)", result.Timestamp, time.Since(result.Timestamp))
	}
}

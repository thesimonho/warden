package service

import (
	"encoding/json"
	"math"
	"testing"
	"time"

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
	// All ParsedEventTypes are now bridged — no unmapped types remain.
	unmappedTypes := []agent.ParsedEventType{}

	for _, typ := range unmappedTypes {
		event := agent.ParsedEvent{Type: typ, SessionID: "s1", Timestamp: "2026-01-01T00:00:00Z"}
		result := SessionEventToContainerEvent(event, ctx)
		if result != nil {
			t.Errorf("expected nil for %s, got %+v", typ, result)
		}
	}
}

func TestSessionEventToContainerEvent_ToolUsePayload(t *testing.T) {
	t.Parallel()

	ctx := SessionContext{ProjectID: "proj-1", ContainerName: "warden-test", WorktreeID: "feat-x"}
	event := agent.ParsedEvent{
		Type:      agent.EventToolUse,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00.000Z",
		ToolName:  "Bash",
		ToolInput: "ls -la",
	}

	result := SessionEventToContainerEvent(event, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != eventbus.EventToolUse {
		t.Errorf("Type = %q, want %q", result.Type, eventbus.EventToolUse)
	}

	var data eventbus.ToolUseData
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
	event := agent.ParsedEvent{
		Type:      agent.EventUserPrompt,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
		Prompt:    "fix the bug in main.go",
	}

	result := SessionEventToContainerEvent(event, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != eventbus.EventUserPrompt {
		t.Errorf("Type = %q, want %q", result.Type, eventbus.EventUserPrompt)
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
	event := agent.ParsedEvent{
		Type:             agent.EventTokenUpdate,
		SessionID:        "sess-123",
		Timestamp:        "2026-03-30T10:00:00Z",
		EstimatedCostUSD: 0.0042,
	}

	result := SessionEventToContainerEvent(event, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	// TokenUpdate maps to EventCostUpdate for the cost persistence pipeline.
	if result.Type != eventbus.EventCostUpdate {
		t.Errorf("Type = %q, want %q (TokenUpdate maps to CostUpdate)", result.Type, eventbus.EventCostUpdate)
	}

	var data eventbus.CostData
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
	event := agent.ParsedEvent{
		Type:      agent.EventSessionStart,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
	}

	result := SessionEventToContainerEvent(event, ctx)
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Type != eventbus.EventSessionStart {
		t.Errorf("Type = %q, want %q", result.Type, eventbus.EventSessionStart)
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
	event := agent.ParsedEvent{
		Type:      agent.EventToolUse,
		SessionID: "sess-1",
		Timestamp: "2026-03-30T10:00:00Z",
		ToolName:  "Read",
	}

	result := SessionEventToContainerEvent(event, ctx)
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
	event := agent.ParsedEvent{
		Type:      agent.EventSessionStart,
		Timestamp: "2026-03-30T10:15:30.123456789Z",
	}
	result := SessionEventToContainerEvent(event, ctx)
	if result.Timestamp.Year() != 2026 || result.Timestamp.Month() != 3 || result.Timestamp.Day() != 30 {
		t.Errorf("RFC3339Nano: timestamp = %v, want 2026-03-30", result.Timestamp)
	}

	// Millisecond format (fallback).
	event.Timestamp = "2026-03-30T10:15:30.123Z"
	result = SessionEventToContainerEvent(event, ctx)
	if result.Timestamp.Year() != 2026 {
		t.Errorf("millisecond format: timestamp = %v, want year 2026", result.Timestamp)
	}

	// Invalid format falls back to approximately now.
	event.Timestamp = "not-a-timestamp"
	result = SessionEventToContainerEvent(event, ctx)
	if time.Since(result.Timestamp) > 5*time.Second {
		t.Errorf("invalid timestamp: expected ~now, got %v (diff %v)", result.Timestamp, time.Since(result.Timestamp))
	}
}

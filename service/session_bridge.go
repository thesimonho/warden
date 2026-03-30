package service

import (
	"encoding/json"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/eventbus"
)

// SessionEventToContainerEvent converts a ParsedEvent from the JSONL parser
// into a ContainerEvent for the event pipeline (store → broker → SSE →
// frontend, audit log). Returns nil for events that don't map to container
// event types.
func SessionEventToContainerEvent(event agent.ParsedEvent, projectID, containerName, worktreeID string) *eventbus.ContainerEvent {
	eventType := mapEventType(event.Type)
	if eventType == "" {
		return nil
	}

	ce := &eventbus.ContainerEvent{
		Type:          eventType,
		ContainerName: containerName,
		ProjectID:     projectID,
		WorktreeID:    worktreeID,
		Timestamp:     parseTimestamp(event.Timestamp),
	}

	// Attach event-specific data payloads.
	switch event.Type {
	case agent.EventToolUse:
		ce.Data = marshalData(eventbus.ToolUseData{
			ToolName:  event.ToolName,
			ToolInput: event.ToolInput,
		})
	case agent.EventUserPrompt:
		// User prompt text goes in the ContainerEvent as data with a "prompt" field.
		ce.Data = marshalData(map[string]string{"prompt": event.Prompt})
	case agent.EventTokenUpdate:
		ce.Data = marshalData(eventbus.CostData{
			TotalCost:   event.EstimatedCostUSD,
			IsEstimated: true,
			SessionID:   event.SessionID,
		})
	}

	return ce
}

// mapEventType converts a ParsedEventType to a ContainerEventType.
// Returns "" for event types that don't map to container events.
func mapEventType(parsed agent.ParsedEventType) eventbus.ContainerEventType {
	switch parsed {
	case agent.EventSessionStart:
		return eventbus.EventSessionStart
	case agent.EventSessionEnd:
		return eventbus.EventSessionEnd
	case agent.EventToolUse:
		return eventbus.EventToolUse
	case agent.EventUserPrompt:
		return eventbus.EventUserPrompt
	case agent.EventTurnComplete:
		return eventbus.EventStop
	case agent.EventTokenUpdate:
		// Token updates carry cost data and are mapped to stop events so
		// they flow through the existing cost persistence pipeline.
		return eventbus.EventStop
	default:
		// TurnDuration — informational, not forwarded to the event pipeline.
		return ""
	}
}

// parseTimestamp converts an ISO 8601 timestamp string to time.Time.
func parseTimestamp(ts string) time.Time {
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		// Try without nano precision.
		t, err = time.Parse("2006-01-02T15:04:05.000Z", ts)
		if err != nil {
			return time.Now().UTC()
		}
	}
	return t
}

// marshalData marshals data to json.RawMessage, returning nil on error.
func marshalData(v any) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	return data
}

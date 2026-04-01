package service

import (
	"encoding/json"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/eventbus"
)

// SessionContext identifies the project and worktree a parsed event belongs to.
type SessionContext struct {
	ProjectID     string
	ContainerName string
	WorktreeID    string
}

// SessionEventToContainerEvent converts a ParsedEvent from the JSONL parser
// into a ContainerEvent for the event pipeline (store → broker → SSE →
// frontend, audit log). Returns nil for events that don't map to container
// event types.
func SessionEventToContainerEvent(event agent.ParsedEvent, ctx SessionContext) *eventbus.ContainerEvent {
	eventType := mapEventType(event.Type)
	if eventType == "" {
		return nil
	}

	ce := &eventbus.ContainerEvent{
		Type:          eventType,
		ContainerName: ctx.ContainerName,
		ProjectID:     ctx.ProjectID,
		WorktreeID:    ctx.WorktreeID,
		Timestamp:     parseTimestamp(event.Timestamp),
		SourceLine:    event.SourceLine,
		SourceIndex:   event.SourceIndex,
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
	case agent.EventToolUseFailure:
		ce.Data = marshalData(eventbus.ToolUseFailureData{
			ToolName: event.ToolName,
			Error:    event.ErrorContent,
		})
	case agent.EventStopFailure:
		ce.Data = marshalData(eventbus.StopFailureData{
			Error: event.ErrorContent,
		})
	case agent.EventPermissionRequest:
		ce.Data = marshalData(eventbus.PermissionRequestData{
			ToolName: event.ToolName,
		})
	case agent.EventElicitation:
		ce.Data = marshalData(eventbus.ElicitationData{
			MCPServerName: event.ServerName,
		})
	case agent.EventTurnDuration:
		ce.Data = marshalData(eventbus.TurnDurationData{
			DurationMs: event.DurationMs,
		})
	case agent.EventSubagentStop:
		ce.Data = marshalData(eventbus.SubagentData{})
	case agent.EventApiMetrics:
		ce.Data = marshalData(eventbus.ApiMetricsData{
			TTFTMs:             event.TTFTMs,
			OutputTokensPerSec: event.OutputTokensPerSec,
		})
	case agent.EventPermissionGrant:
		ce.Data = marshalData(eventbus.PermissionGrantData{
			Commands: event.Commands,
		})
	case agent.EventContextCompact:
		ce.Data = marshalData(eventbus.ContextCompactData{
			Trigger:   event.CompactTrigger,
			PreTokens: event.PreCompactTokens,
		})
	case agent.EventSystemInfo:
		ce.Data = marshalData(eventbus.SystemInfoData{
			Subtype: event.Subtype,
			Content: event.Content,
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
	case agent.EventTokenUpdate:
		// Token updates carry cost data and are mapped to stop events so
		// they flow through the existing cost persistence pipeline.
		return eventbus.EventStop
	case agent.EventToolUseFailure:
		return eventbus.EventToolUseFailure
	case agent.EventStopFailure:
		return eventbus.EventStopFailure
	case agent.EventPermissionRequest:
		return eventbus.EventPermissionRequest
	case agent.EventElicitation:
		return eventbus.EventElicitation
	case agent.EventSubagentStop:
		return eventbus.EventSubagentStop
	case agent.EventApiMetrics:
		return eventbus.EventApiMetrics
	case agent.EventPermissionGrant:
		return eventbus.EventPermissionGrant
	case agent.EventContextCompact:
		return eventbus.EventContextCompact
	case agent.EventSystemInfo:
		return eventbus.EventSystemInfo
	case agent.EventTurnComplete:
		return eventbus.EventTurnComplete
	case agent.EventTurnDuration:
		return eventbus.EventTurnDuration
	default:
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

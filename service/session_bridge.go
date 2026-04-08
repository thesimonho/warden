package service

import (
	"encoding/json"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/event"
)

// SessionContext identifies the project and worktree a parsed event belongs to.
type SessionContext struct {
	ProjectID     string
	ContainerName string
	AgentType     string
	WorktreeID    string
}

// SessionEventToContainerEvent converts a ParsedEvent from the JSONL parser
// into a ContainerEvent for the event pipeline (store → broker → SSE →
// frontend, audit log). Returns nil for events that don't map to container
// event types.
func SessionEventToContainerEvent(parsed agent.ParsedEvent, ctx SessionContext) *event.ContainerEvent {
	eventType := mapEventType(parsed.Type)
	if eventType == "" {
		return nil
	}

	ce := &event.ContainerEvent{
		Type:          eventType,
		ContainerName: ctx.ContainerName,
		ProjectID:     ctx.ProjectID,
		AgentType:     ctx.AgentType,
		WorktreeID:    ctx.WorktreeID,
		Timestamp:     parseTimestamp(parsed.Timestamp),
		Source:        event.SourceJSONL,
		SourceLine:    parsed.SourceLine,
		SourceIndex:   parsed.SourceIndex,
	}

	// Attach event-specific data payloads.
	switch parsed.Type {
	case event.EventToolUse:
		ce.Data = marshalData(event.ToolUseData{
			ToolName:  parsed.ToolName,
			ToolInput: parsed.ToolInput,
		})
	case event.EventUserPrompt:
		promptSource := ""
		if parsed.PromptSource.IsBash() {
			promptSource = string(parsed.PromptSource)
		}
		ce.Data = marshalData(event.UserPromptData{
			Prompt:       parsed.Prompt,
			PromptSource: promptSource,
		})
	case event.EventTokenUpdate:
		ce.Data = marshalData(event.CostData{
			TotalCost:   parsed.EstimatedCostUSD,
			IsEstimated: true,
			SessionID:   parsed.SessionID,
		})
	case event.EventToolUseFailure:
		ce.Data = marshalData(event.ToolUseFailureData{
			ToolName: parsed.ToolName,
			Error:    parsed.ErrorContent,
		})
	case event.EventStopFailure:
		ce.Data = marshalData(event.StopFailureData{
			Error: parsed.ErrorContent,
		})
	case event.EventPermissionRequest:
		ce.Data = marshalData(event.PermissionRequestData{
			ToolName: parsed.ToolName,
		})
	case event.EventElicitation:
		ce.Data = marshalData(event.ElicitationData{
			MCPServerName: parsed.ServerName,
		})
	case event.EventTurnDuration:
		ce.Data = marshalData(event.TurnDurationData{
			DurationMs: parsed.DurationMs,
		})
	case event.EventSubagentStop:
		ce.Data = marshalData(event.SubagentData{})
	case event.EventApiMetrics:
		ce.Data = marshalData(event.ApiMetricsData{
			TTFTMs:             parsed.TTFTMs,
			OutputTokensPerSec: parsed.OutputTokensPerSec,
		})
	case event.EventPermissionGrant:
		ce.Data = marshalData(event.PermissionGrantData{
			Commands: parsed.Commands,
		})
	case event.EventContextCompact:
		ce.Data = marshalData(event.ContextCompactData{
			Trigger:   parsed.CompactTrigger,
			PreTokens: parsed.PreCompactTokens,
		})
	case event.EventSystemInfo:
		ce.Data = marshalData(event.SystemInfoData{
			Subtype: parsed.Subtype,
			Content: parsed.Content,
		})
	}

	return ce
}

// mapEventType converts a ParsedEventType to a ContainerEventType.
// Since both are now the same underlying type ([event.ContainerEventType]),
// most types pass through as-is. Only EventTokenUpdate requires remapping
// to EventCostUpdate. Returns "" for unknown event types.
func mapEventType(parsed event.ContainerEventType) event.ContainerEventType {
	if parsed == event.EventTokenUpdate {
		// Token updates carry cumulative cost data and are mapped to cost
		// update events for the cost persistence + budget enforcement pipeline.
		return event.EventCostUpdate
	}
	if !event.IsKnownType(parsed) {
		return ""
	}
	return parsed
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

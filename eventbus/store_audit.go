package eventbus

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"strconv"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/event"
)

// writeToAuditLog persists a container event to the audit log via the
// AuditWriter. Skips events that are too noisy or have no independent
// audit value:
//   - heartbeat: fires every 30s per container
//   - attention_clear: fires on every user prompt (user_prompt captures this)
//   - cost_update: fires on every assistant message with token usage; cost data
//     is already persisted via handleCostUpdate → PersistSessionCost, so the
//     audit entry adds noise without value
func (s *Store) writeToAuditLog(writer *db.AuditWriter, evt event.ContainerEvent) {
	if writer == nil || evt.Type == event.EventHeartbeat || evt.Type == event.EventAttentionClear || evt.Type == event.EventCostUpdate || evt.Type == event.EventRuntimeInstalling || evt.Type == event.EventAgentInstalling {
		return
	}

	source := db.SourceAgent
	eventName := string(evt.Type)
	var message string

	switch evt.Type {
	case event.EventTerminalConnected, event.EventTerminalDisconnected, event.EventProcessKilled, event.EventSessionExit:
		source = db.SourceContainer
	case event.EventContainerError:
		source = db.SourceContainer
		if evt.Data != nil {
			var data event.ContainerErrorData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.Message
			}
		}
	case event.EventNetworkBlocked:
		source = db.SourceContainer
		if evt.Data != nil {
			var data event.NetworkBlockedData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				if data.Domain != "" {
					message = data.Domain + " (" + data.IP + ")"
				} else {
					message = data.IP
				}
			}
		}
	case event.EventToolUse:
		if evt.Data != nil {
			var data event.ToolUseData
			if err := json.Unmarshal(evt.Data, &data); err == nil && data.ToolName != "" {
				message = data.ToolName
			}
		}
	case event.EventToolUseFailure:
		if evt.Data != nil {
			var data event.ToolUseFailureData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.ToolName
			}
		}
	case event.EventStopFailure:
		if evt.Data != nil {
			var data event.StopFailureData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.Error
			}
		}
	case event.EventPermissionRequest:
		if evt.Data != nil {
			var data event.PermissionRequestData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.ToolName
			}
		}
	case event.EventSubagentStart, event.EventSubagentStop:
		if evt.Data != nil {
			var data event.SubagentData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.AgentType
			}
		}
	case event.EventConfigChange:
		if evt.Data != nil {
			var data event.ConfigChangeData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.Source
			}
		}
	case event.EventInstructionsLoaded:
		if evt.Data != nil {
			var data event.InstructionsLoadedData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.FilePath
			}
		}
	case event.EventTaskCompleted:
		if evt.Data != nil {
			var data event.TaskCompletedData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.TaskSubject
			}
		}
	case event.EventElicitation, event.EventElicitationResult:
		if evt.Data != nil {
			var data event.ElicitationData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				message = data.MCPServerName
			}
		}
	case event.EventUserPrompt:
		if evt.Data != nil {
			var data event.UserPromptData
			if err := json.Unmarshal(evt.Data, &data); err == nil {
				prefix := ""
				if data.IsBash() {
					prefix = "[bash] "
				}
				message = prefix + data.Prompt
			}
		}
	}

	level := db.LevelInfo
	switch evt.Type {
	case event.EventToolUseFailure, event.EventStopFailure, event.EventNetworkBlocked, event.EventContainerError:
		level = db.LevelError
	}

	entry := db.Entry{
		Timestamp:     evt.Timestamp,
		Source:        source,
		Level:         level,
		ProjectID:     evt.ProjectID,
		AgentType:     evt.AgentType,
		ContainerName: evt.ContainerName,
		Worktree:      evt.WorktreeID,
		Event:         eventName,
		Message:       message,
		Data:          evt.Data,
	}

	// Compute content hash for JSONL-sourced event dedup. Only JSONL events
	// carry SourceLine bytes; event-dir and backend events are inherently
	// unique (one file per event, one call per action).
	if evt.Source == event.SourceJSONL && len(evt.SourceLine) > 0 {
		h := fnv.New64a()
		h.Write(evt.SourceLine)
		h.Write([]byte(strconv.Itoa(evt.SourceIndex)))
		entry.SourceID = hex.EncodeToString(h.Sum(nil))
	}

	writer.Write(entry)
}

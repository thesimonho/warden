package eventbus

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"strconv"

	"github.com/thesimonho/warden/db"
)

// writeToAuditLog persists a container event to the audit log via the
// AuditWriter. Skips events that are too noisy or have no independent
// audit value:
//   - heartbeat: fires every 30s per container
//   - attention_clear: fires on every user prompt (user_prompt captures this)
//   - cost_update: fires on every assistant message with token usage; cost data
//     is already persisted via handleCostUpdate → PersistSessionCost, so the
//     audit entry adds noise without value
func (s *Store) writeToAuditLog(writer *db.AuditWriter, event ContainerEvent) {
	if writer == nil || event.Type == EventHeartbeat || event.Type == EventAttentionClear || event.Type == EventCostUpdate || event.Type == EventRuntimeInstalling || event.Type == EventAgentInstalling {
		return
	}

	source := db.SourceAgent
	eventName := string(event.Type)
	var message string

	switch event.Type {
	case EventTerminalConnected, EventTerminalDisconnected, EventProcessKilled, EventSessionExit:
		source = db.SourceContainer
	case EventNetworkBlocked:
		source = db.SourceContainer
		if event.Data != nil {
			var data NetworkBlockedData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				if data.Domain != "" {
					message = data.Domain + " (" + data.IP + ")"
				} else {
					message = data.IP
				}
			}
		}
	case EventToolUse:
		if event.Data != nil {
			var data ToolUseData
			if err := json.Unmarshal(event.Data, &data); err == nil && data.ToolName != "" {
				message = data.ToolName
			}
		}
	case EventToolUseFailure:
		if event.Data != nil {
			var data ToolUseFailureData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.ToolName
			}
		}
	case EventStopFailure:
		if event.Data != nil {
			var data StopFailureData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.Error
			}
		}
	case EventPermissionRequest:
		if event.Data != nil {
			var data PermissionRequestData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.ToolName
			}
		}
	case EventSubagentStart, EventSubagentStop:
		if event.Data != nil {
			var data SubagentData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.AgentType
			}
		}
	case EventConfigChange:
		if event.Data != nil {
			var data ConfigChangeData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.Source
			}
		}
	case EventInstructionsLoaded:
		if event.Data != nil {
			var data InstructionsLoadedData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.FilePath
			}
		}
	case EventTaskCompleted:
		if event.Data != nil {
			var data TaskCompletedData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.TaskSubject
			}
		}
	case EventElicitation, EventElicitationResult:
		if event.Data != nil {
			var data ElicitationData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				message = data.MCPServerName
			}
		}
	case EventUserPrompt:
		if event.Data != nil {
			var data UserPromptData
			if err := json.Unmarshal(event.Data, &data); err == nil {
				prefix := ""
				if data.IsBash() {
					prefix = "[bash] "
				}
				message = prefix + data.Prompt
			}
		}
	}

	level := db.LevelInfo
	switch event.Type {
	case EventToolUseFailure, EventStopFailure, EventNetworkBlocked:
		level = db.LevelError
	}

	entry := db.Entry{
		Timestamp:     event.Timestamp,
		Source:        source,
		Level:         level,
		ProjectID:     event.ProjectID,
		AgentType:     event.AgentType,
		ContainerName: event.ContainerName,
		Worktree:      event.WorktreeID,
		Event:         eventName,
		Message:       message,
		Data:          event.Data,
	}

	// Compute content hash for JSONL-sourced event dedup.
	if len(event.SourceLine) > 0 {
		h := fnv.New64a()
		h.Write(event.SourceLine)
		h.Write([]byte(strconv.Itoa(event.SourceIndex)))
		entry.SourceID = hex.EncodeToString(h.Sum(nil))
	}

	writer.Write(entry)
}

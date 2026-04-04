// Package eventbus implements a file-based event system for container-to-host
// communication. Containers write JSON event files to a bind-mounted directory;
// the Go backend watches for new files, maintains in-memory state, and
// broadcasts changes to SSE-connected frontends.
package eventbus

import (
	"encoding/json"
	"time"

	"github.com/thesimonho/warden/engine"
)

// ContainerEventType identifies the kind of event from a container.
type ContainerEventType string

const (
	// EventAttention means Claude needs user attention (permission, question, idle).
	EventAttention ContainerEventType = "attention"
	// EventAttentionClear means the user has responded or Claude resumed work.
	EventAttentionClear ContainerEventType = "attention_clear"
	// EventSessionStart means a new Claude session started.
	EventSessionStart ContainerEventType = "session_start"
	// EventSessionEnd means a Claude session ended.
	EventSessionEnd ContainerEventType = "session_end"
	// EventCostUpdate carries cumulative cost data parsed from agent JSONL token usage.
	EventCostUpdate ContainerEventType = "cost_update"
	// EventNeedsAnswer means Claude is asking the user a question (AskUserQuestion tool).
	EventNeedsAnswer ContainerEventType = "needs_answer"
	// EventUserPrompt logs the user's prompt text (from UserPromptSubmit hook).
	EventUserPrompt ContainerEventType = "user_prompt"
	// EventHeartbeat is a periodic keepalive from the container.
	EventHeartbeat ContainerEventType = "heartbeat"
	// EventTerminalConnected means a tmux session was created for a worktree.
	EventTerminalConnected ContainerEventType = "terminal_connected"
	// EventTerminalDisconnected means the terminal viewer was disconnected.
	EventTerminalDisconnected ContainerEventType = "terminal_disconnected"
	// EventProcessKilled means the tmux session was killed — the worktree is fully dead.
	EventProcessKilled ContainerEventType = "process_killed"
	// EventSessionExit means the agent exited inside a terminal (exit code available).
	EventSessionExit ContainerEventType = "session_exit"
	// EventToolUse is logged for every PreToolUse hook event, capturing the tool name and input.
	EventToolUse ContainerEventType = "tool_use"
	// EventToolUseFailure is logged when a tool execution fails (PostToolUseFailure hook).
	EventToolUseFailure ContainerEventType = "tool_use_failure"
	// EventStopFailure is logged when a turn ends due to an API error (StopFailure hook).
	EventStopFailure ContainerEventType = "stop_failure"
	// EventPermissionRequest is logged when Claude is about to show a permission dialog.
	EventPermissionRequest ContainerEventType = "permission_request"
	// EventSubagentStart is logged when a subagent is spawned via the Agent tool.
	EventSubagentStart ContainerEventType = "subagent_start"
	// EventSubagentStop is logged when a subagent finishes responding.
	EventSubagentStop ContainerEventType = "subagent_stop"
	// EventConfigChange is logged when a configuration file changes during a session.
	EventConfigChange ContainerEventType = "config_change"
	// EventInstructionsLoaded is logged when CLAUDE.md or rules files are loaded.
	EventInstructionsLoaded ContainerEventType = "instructions_loaded"
	// EventTaskCompleted is logged when a task is marked complete.
	EventTaskCompleted ContainerEventType = "task_completed"
	// EventElicitation is logged when an MCP server requests user input.
	EventElicitation ContainerEventType = "elicitation"
	// EventElicitationResult is logged after a user responds to an MCP elicitation.
	EventElicitationResult ContainerEventType = "elicitation_result"
	// EventTurnComplete is logged when an agent turn ends (stop_reason: end_turn).
	EventTurnComplete ContainerEventType = "turn_complete"
	// EventTurnDuration is logged with the wall-clock duration of an agent turn.
	EventTurnDuration ContainerEventType = "turn_duration"
	// EventApiMetrics records API performance metrics (TTFT, output tokens/sec).
	EventApiMetrics ContainerEventType = "api_metrics"
	// EventPermissionGrant is logged when a permission request is granted.
	EventPermissionGrant ContainerEventType = "permission_grant"
	// EventContextCompact is logged when context window compaction occurs.
	EventContextCompact ContainerEventType = "context_compact"
	// EventSystemInfo is logged for informational system messages from the agent.
	EventSystemInfo ContainerEventType = "system_info"
	// EventRuntimeInstalling is emitted when a language runtime is being installed.
	EventRuntimeInstalling ContainerEventType = "runtime_installing"
	// EventRuntimeInstalled is emitted when a language runtime has been installed.
	EventRuntimeInstalled ContainerEventType = "runtime_installed"
	// EventAgentInstalling is emitted when an agent CLI is being installed/updated.
	EventAgentInstalling ContainerEventType = "agent_installing"
	// EventAgentInstalled is emitted when an agent CLI installation completes.
	EventAgentInstalled ContainerEventType = "agent_installed"
	// EventNetworkBlocked is emitted when an outbound connection is rejected by network isolation.
	EventNetworkBlocked ContainerEventType = "network_blocked"
)

// ContainerEvent is the JSON payload written by container hook scripts
// to the bind-mounted event directory.
type ContainerEvent struct {
	// Type identifies the event kind.
	Type ContainerEventType `json:"type"`
	// ContainerName is the Docker container name (set via WARDEN_CONTAINER_NAME env).
	ContainerName string `json:"containerName"`
	// ProjectID is the deterministic project identifier (set via WARDEN_PROJECT_ID env).
	ProjectID string `json:"projectId,omitempty"`
	// AgentType identifies the agent that produced this event (set via WARDEN_AGENT_TYPE env).
	AgentType string `json:"agentType,omitempty"`
	// WorktreeID is the worktree this event pertains to (e.g. "main", "feature-x").
	WorktreeID string `json:"worktreeId"`
	// Data carries event-specific payload (e.g. notificationType, cost fields).
	Data json.RawMessage `json:"data,omitempty"`
	// Timestamp is when the event was created (set by the container hook script).
	Timestamp time.Time `json:"timestamp"`
	// SourceLine is the raw JSONL line bytes for dedup hashing.
	// Only set for events sourced from JSONL session files.
	SourceLine []byte `json:"-"`
	// SourceIndex disambiguates multiple events parsed from the same JSONL line.
	SourceIndex int `json:"-"`
}

// Ref returns a [ProjectRef] from this event's identity fields.
func (e ContainerEvent) Ref() ProjectRef {
	return ProjectRef{
		ProjectID:     e.ProjectID,
		AgentType:     e.AgentType,
		ContainerName: e.ContainerName,
	}
}

// AttentionData carries notification details for attention events.
type AttentionData struct {
	NotificationType engine.NotificationType `json:"notificationType"`
}

// CostData carries cost information from the stop event.
type CostData struct {
	TotalCost    float64 `json:"totalCost"`
	MessageCount int     `json:"messageCount"`
	IsEstimated  bool    `json:"isEstimated"`
	SessionID    string  `json:"sessionId,omitempty"`
}

// TerminalConnectedData is the payload for terminal_connected events.
// Currently empty — retained as a type for future extension.
type TerminalConnectedData struct{}

// SessionExitData carries the exit code from a session_exit event.
type SessionExitData struct {
	ExitCode int `json:"exitCode"`
}

// ToolUseData carries tool invocation details from the PreToolUse hook.
type ToolUseData struct {
	ToolName  string `json:"toolName"`
	ToolInput string `json:"toolInput,omitempty"`
}

// ToolUseFailureData carries details when a tool execution fails.
type ToolUseFailureData struct {
	ToolName string `json:"toolName"`
	Error    string `json:"error,omitempty"`
}

// StopFailureData carries details when a turn ends due to an API error.
type StopFailureData struct {
	Error        string `json:"error"`
	ErrorDetails string `json:"errorDetails,omitempty"`
}

// PermissionRequestData carries details about a permission dialog.
type PermissionRequestData struct {
	ToolName string `json:"toolName"`
}

// SubagentData carries details about subagent lifecycle events.
type SubagentData struct {
	AgentID   string `json:"agentId,omitempty"`
	AgentType string `json:"agentType,omitempty"`
}

// ConfigChangeData carries details about configuration file changes.
type ConfigChangeData struct {
	Source   string `json:"source"`
	FilePath string `json:"filePath,omitempty"`
}

// InstructionsLoadedData carries details about loaded instruction files.
type InstructionsLoadedData struct {
	FilePath   string `json:"filePath"`
	LoadReason string `json:"loadReason,omitempty"`
}

// TaskCompletedData carries details about task completion.
type TaskCompletedData struct {
	TaskID      string `json:"taskId,omitempty"`
	TaskSubject string `json:"taskSubject,omitempty"`
}

// ElicitationData carries details about MCP elicitation events.
type ElicitationData struct {
	MCPServerName string `json:"mcpServerName,omitempty"`
	Action        string `json:"action,omitempty"`
}

// TurnDurationData carries the wall-clock duration of an agent turn.
type TurnDurationData struct {
	DurationMs int64 `json:"durationMs"`
}

// ApiMetricsData carries API performance metrics from the agent.
type ApiMetricsData struct {
	TTFTMs             float64 `json:"ttftMs"`
	OutputTokensPerSec float64 `json:"outputTokensPerSec"`
}

// PermissionGrantData carries details about a granted permission.
type PermissionGrantData struct {
	Commands []string `json:"commands,omitempty"`
}

// ContextCompactData carries details about context window compaction.
type ContextCompactData struct {
	Trigger   string `json:"trigger,omitempty"`
	PreTokens int64  `json:"preTokens,omitempty"`
}

// SystemInfoData carries details for informational system messages.
type SystemInfoData struct {
	Subtype string `json:"subtype"`
	Content string `json:"content,omitempty"`
}

// RuntimeStatusData carries details about runtime installation progress.
type RuntimeStatusData struct {
	RuntimeID    string `json:"runtimeId"`
	RuntimeLabel string `json:"runtimeLabel"`
}

// RuntimeStatusPayload is the SSE payload for runtime install progress.
type RuntimeStatusPayload struct {
	ProjectRef
	Phase        string `json:"phase"`
	RuntimeID    string `json:"runtimeId"`
	RuntimeLabel string `json:"runtimeLabel"`
}

// NetworkBlockedData carries details about a blocked outbound connection.
type NetworkBlockedData struct {
	IP     string `json:"ip"`
	Domain string `json:"domain,omitempty"`
}

// AgentStatusData carries details about agent CLI installation progress.
type AgentStatusData struct {
	Version string `json:"version"`
}

// AgentStatusPayload is the SSE payload for agent CLI install progress.
type AgentStatusPayload struct {
	ProjectRef
	Phase   string `json:"phase"`
	Version string `json:"version"`
}

// SSEEventType identifies the kind of event sent to frontend clients.
type SSEEventType string

const (
	// SSEWorktreeState is sent when a worktree's attention/session state changes.
	SSEWorktreeState SSEEventType = "worktree_state"
	// SSEProjectState is sent when project-level data changes (e.g. cost).
	SSEProjectState SSEEventType = "project_state"
	// SSEWorktreeListChanged is sent when the worktree list changes (create, remove, cleanup, kill).
	SSEWorktreeListChanged SSEEventType = "worktree_list_changed"
	// SSEBudgetExceeded is sent when a project exceeds its cost budget.
	SSEBudgetExceeded SSEEventType = "budget_exceeded"
	// SSEBudgetContainerStopped is sent after a container is stopped due to budget enforcement.
	SSEBudgetContainerStopped SSEEventType = "budget_container_stopped"
	// SSEHeartbeat is a keepalive sent at regular intervals.
	SSEHeartbeat SSEEventType = "heartbeat"
	// SSERuntimeStatus is sent when a runtime installation starts or completes.
	SSERuntimeStatus SSEEventType = "runtime_status"
	// SSEAgentStatus is sent when an agent CLI installation starts or completes.
	SSEAgentStatus SSEEventType = "agent_status"
)

// ProjectRef identifies a project in SSE event payloads. Embedded by all
// broadcast payload structs so the frontend can match events to the correct
// (projectId, agentType) pair. ContainerName is included for display and
// legacy matching.
type ProjectRef struct {
	ProjectID     string `json:"projectId,omitempty"`
	AgentType     string `json:"agentType,omitempty"`
	ContainerName string `json:"containerName"`
}

// BudgetEventPayload is the shared JSON payload for budget enforcement SSE
// events (budget_exceeded and budget_container_stopped).
type BudgetEventPayload struct {
	ProjectRef
	TotalCost float64 `json:"totalCost"`
	Budget    float64 `json:"budget"`
}

// BudgetContainerStoppedPayload extends [BudgetEventPayload] with the
// container ID so frontends can match against a URL-based project ID
// without needing a project list lookup.
type BudgetContainerStoppedPayload struct {
	BudgetEventPayload
	ContainerID string `json:"containerId"`
}

// SSEEvent is a typed event sent to frontend clients over Server-Sent Events.
type SSEEvent struct {
	// Event is the SSE event name (used in the `event:` field).
	Event SSEEventType `json:"event"`
	// Data is the JSON-encoded payload (used in the `data:` field).
	Data json.RawMessage `json:"data"`
}

package event

import "encoding/json"

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
	// SSEServerShutdown is sent immediately before the server stops.
	// Frontends should show a "Warden stopped" state. Integrators can
	// use this to trigger cleanup or reconnection logic.
	SSEServerShutdown SSEEventType = "server_shutdown"
	// SSEHeartbeat is a keepalive sent at regular intervals.
	SSEHeartbeat SSEEventType = "heartbeat"
	// SSERuntimeStatus is sent when a runtime installation starts or completes.
	SSERuntimeStatus SSEEventType = "runtime_status"
	// SSEAgentStatus is sent when an agent CLI installation starts or completes.
	SSEAgentStatus SSEEventType = "agent_status"
	// SSEContainerStateChanged is sent when a container is created, started,
	// stopped, or deleted. The tray uses this to track running containers
	// without polling.
	SSEContainerStateChanged SSEEventType = "container_state_changed"
	// SSEViewerFocus is sent when the aggregated viewer focus state changes.
	// The tray uses this to suppress notifications for projects the user
	// is actively viewing.
	SSEViewerFocus SSEEventType = "viewer_focus"
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

// RuntimeStatusPayload is the SSE payload for runtime install progress.
type RuntimeStatusPayload struct {
	ProjectRef
	Phase        string `json:"phase"`
	RuntimeID    string `json:"runtimeId"`
	RuntimeLabel string `json:"runtimeLabel"`
}

// AgentStatusPayload is the SSE payload for agent CLI install progress.
type AgentStatusPayload struct {
	ProjectRef
	Phase   string `json:"phase"`
	Version string `json:"version"`
}

// ContainerStateAction identifies what happened to a container.
type ContainerStateAction string

const (
	// ContainerActionCreated means a new container was created.
	ContainerActionCreated ContainerStateAction = "created"
	// ContainerActionStarted means a container was started (or restarted).
	ContainerActionStarted ContainerStateAction = "started"
	// ContainerActionStopped means a container was stopped.
	ContainerActionStopped ContainerStateAction = "stopped"
	// ContainerActionDeleted means a container was removed.
	ContainerActionDeleted ContainerStateAction = "deleted"
)

// ContainerStatePayload is the SSE payload for container lifecycle events.
type ContainerStatePayload struct {
	ProjectRef
	Action ContainerStateAction `json:"action"`
}

// SSEEvent is a typed event sent to frontend clients over Server-Sent Events.
type SSEEvent struct {
	// Event is the SSE event name (used in the `event:` field).
	Event SSEEventType `json:"event"`
	// Data is the JSON-encoded payload (used in the `data:` field).
	Data json.RawMessage `json:"data"`
}

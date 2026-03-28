package eventbus

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

// WorktreeState holds the real-time state for a single worktree,
// derived from container hook events pushed via the event bus.
type WorktreeState struct {
	// NeedsInput is true when Claude is waiting for user attention.
	NeedsInput bool
	// NotificationType indicates why attention is needed.
	NotificationType engine.NotificationType
	// SessionActive is true when a Claude session is running in this worktree.
	SessionActive bool
	// UpdatedAt is when this state was last changed.
	UpdatedAt time.Time
}

// ProjectCost holds accumulated cost for a container.
type ProjectCost struct {
	// TotalCost is the aggregate USD cost across all sessions.
	TotalCost float64
	// MessageCount is the total number of messages.
	MessageCount int
	// IsEstimated is true when the cost is an estimate (subscription user).
	IsEstimated bool
	// UpdatedAt is when cost was last reported.
	UpdatedAt time.Time
}

// worktreeKey uniquely identifies a worktree within a container.
type worktreeKey struct {
	containerName string
	worktreeID    string
}

// pendingBroadcast holds an SSE event to be sent after the store lock is released.
type pendingBroadcast struct {
	event SSEEventType
	data  any
}

// TerminalState holds push-based terminal lifecycle data for a worktree.
type TerminalState struct {
	// AbducoAlive is true when the abduco session is running.
	AbducoAlive bool
	// ViewerConnected is true when a browser is connected via WebSocket.
	ViewerConnected bool
	// ExitCode is Claude's exit code (-1 means not set / still running).
	ExitCode int
	// UpdatedAt is when this state was last changed.
	UpdatedAt time.Time
}

// DeriveWorktreeState maps terminal process liveness to a WorktreeState.
// Returns empty string if the terminal state has never been set.
func (ts *TerminalState) DeriveWorktreeState() engine.WorktreeState {
	if ts == nil || ts.UpdatedAt.IsZero() {
		return ""
	}
	if ts.AbducoAlive {
		if !ts.ViewerConnected {
			return engine.WorktreeStateBackground
		}
		if ts.ExitCode >= 0 {
			return engine.WorktreeStateShell
		}
		return engine.WorktreeStateConnected
	}
	return engine.WorktreeStateDisconnected
}

// WorktreeStatePayload is the JSON shape sent over SSE for worktree_state events.
// Shared by all broadcast helpers to keep the Go and TypeScript types in sync.
type WorktreeStatePayload struct {
	ProjectID        string                  `json:"projectId,omitempty"`
	ContainerName    string                  `json:"containerName"`
	WorktreeID       string                  `json:"worktreeId"`
	NeedsInput       bool                    `json:"needsInput"`
	NotificationType engine.NotificationType `json:"notificationType,omitempty"`
	SessionActive    bool                    `json:"sessionActive"`
	State            engine.WorktreeState    `json:"state,omitempty"`
	ExitCode         int                     `json:"exitCode,omitempty"`
}

// StopCallbackFunc is called on every stop event with any cost data
// parsed from the event payload. sessionID and cost are zero when the
// event carried no cost data. Set via [Store.SetStopCallback].
// projectID is the deterministic project identifier (from WARDEN_PROJECT_ID).
// containerName is the Docker container name (from WARDEN_CONTAINER_NAME).
type StopCallbackFunc func(projectID, containerName, sessionID string, cost float64, isEstimated bool)

// Store holds in-memory state derived from container events.
//
// Thread-safe for concurrent reads from API handlers and writes
// from the file watcher goroutine.
type Store struct {
	mu                 sync.RWMutex
	attention          map[worktreeKey]*WorktreeState
	costs              map[string]*ProjectCost // keyed by containerName
	terminals          map[worktreeKey]*TerminalState
	terminalContainers map[string]struct{}  // keyed by containerName, for O(1) HasTerminalData
	lastEvents         map[string]time.Time // keyed by containerName
	broker             *Broker
	auditWriter        *db.AuditWriter
	onStop             StopCallbackFunc
}

// NewStore creates an empty event store. If broker is non-nil,
// state changes are broadcast to SSE clients. The auditWriter
// handles mode-gated persistence of events to the audit log.
func NewStore(broker *Broker, auditWriter *db.AuditWriter) *Store {
	return &Store{
		attention:          make(map[worktreeKey]*WorktreeState),
		costs:              make(map[string]*ProjectCost),
		terminals:          make(map[worktreeKey]*TerminalState),
		terminalContainers: make(map[string]struct{}),
		lastEvents:         make(map[string]time.Time),
		broker:             broker,
		auditWriter:        auditWriter,
	}
}

// SetStopCallback registers a function called on every stop event.
// The callback receives any cost data from the event (which may be
// zero/empty) and is responsible for both persistence and budget
// enforcement. This is the single integration point between the
// event bus and the service layer's cost/budget system.
func (s *Store) SetStopCallback(fn StopCallbackFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStop = fn
}

// HandleEvent processes a container event, updates state, and
// broadcasts changes to SSE clients. This is the callback passed
// to the Watcher.
func (s *Store) HandleEvent(event ContainerEvent) {
	s.mu.Lock()

	key := worktreeKey{
		containerName: event.ContainerName,
		worktreeID:    event.WorktreeID,
	}
	s.lastEvents[event.ContainerName] = event.Timestamp

	var broadcasts []pendingBroadcast
	var stopCost CostData // populated by handleStop, reused by onStop callback

	switch event.Type {
	case EventAttention:
		broadcasts = s.handleAttention(key, event)
	case EventAttentionClear:
		broadcasts = s.handleAttentionClear(key, event)
	case EventNeedsAnswer:
		broadcasts = s.handleNeedsAnswer(key, event)
	case EventSessionStart:
		broadcasts = s.handleSessionStart(key, event)
	case EventSessionEnd:
		broadcasts = s.handleSessionEnd(key, event)
	case EventStop:
		broadcasts, stopCost = s.handleStop(key, event)
	case EventHeartbeat:
		// No-op — lastEvents is already updated above for all event types.
	case EventUserPrompt:
		// No state change — the prompt is logged to the audit log by writeToAuditLog below.
	case EventToolUse, EventToolUseFailure:
		// No state change — tool events are logged to the audit log by writeToAuditLog below.
	case EventStopFailure:
		// No state change — stop failure is logged for audit.
	case EventPermissionRequest:
		// No state change — permission requests are logged for audit.
	case EventSubagentStart, EventSubagentStop:
		// No state change — subagent lifecycle is logged for audit.
	case EventConfigChange, EventInstructionsLoaded:
		// No state change — config events are logged for audit.
	case EventTaskCompleted:
		// No state change — task completion is logged for audit.
	case EventElicitation, EventElicitationResult:
		// No state change — MCP elicitation events are logged for audit.
	case EventTerminalConnected:
		broadcasts = s.handleTerminalConnected(key, event)
	case EventTerminalDisconnected:
		broadcasts = s.handleTerminalDisconnected(key, event)
	case EventProcessKilled:
		broadcasts = s.handleProcessKilled(key, event)
	case EventSessionExit:
		broadcasts = s.handleSessionExit(key, event)
	default:
		slog.Warn("unknown event type", "type", event.Type, "container", event.ContainerName)
	}

	writer := s.auditWriter
	onStop := s.onStop
	s.mu.Unlock()

	// Write to the audit log (outside the lock).
	s.writeToAuditLog(writer, event)

	// Broadcast outside the lock to avoid holding it during JSON marshal + fan-out.
	s.broadcast(broadcasts)

	// On every stop event, invoke the callback for cost persistence and
	// budget enforcement. Uses cost data already parsed by handleStop.
	// Runs outside the lock because enforcement may call back into docker.
	if event.Type == EventStop && onStop != nil {
		onStop(event.ProjectID, event.ContainerName, stopCost.SessionID, stopCost.TotalCost, stopCost.IsEstimated)
	}
}

// GetWorktreeState returns the attention state for a worktree.
// Returns zero value if no state exists.
func (s *Store) GetWorktreeState(containerName, worktreeID string) WorktreeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := worktreeKey{containerName: containerName, worktreeID: worktreeID}
	if att, ok := s.attention[key]; ok {
		return *att
	}
	return WorktreeState{}
}

// GetProjectCost returns the cost state for a container.
// Returns zero value if no cost data exists.
func (s *Store) GetProjectCost(containerName string) ProjectCost {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if cost, ok := s.costs[containerName]; ok {
		return *cost
	}
	return ProjectCost{}
}

// LastEventTime returns the timestamp of the most recent event
// from the given container. Returns zero time if no events received.
func (s *Store) LastEventTime(containerName string) time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.lastEvents[containerName]
}

// handleAttention sets attention state from a Notification hook event.
func (s *Store) handleAttention(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	var data AttentionData
	if event.Data != nil {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			slog.Warn("invalid attention data", "err", err, "container", event.ContainerName)
		}
	}

	existing := s.attention[key]
	sessionActive := existing != nil && existing.SessionActive

	att := &WorktreeState{
		NeedsInput:       true,
		NotificationType: data.NotificationType,
		SessionActive:    sessionActive,
		UpdatedAt:        event.Timestamp,
	}
	s.attention[key] = att

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, att, s.terminals[key])}
}

// handleAttentionClear clears attention state (user responded or Claude resumed).
func (s *Store) handleAttentionClear(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing, ok := s.attention[key]
	if !ok || !existing.NeedsInput {
		return nil // No change — skip broadcast.
	}

	att := &WorktreeState{
		SessionActive: existing.SessionActive,
		UpdatedAt:     event.Timestamp,
	}
	s.attention[key] = att

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, att, s.terminals[key])}
}

// handleNeedsAnswer sets attention for an AskUserQuestion tool call.
func (s *Store) handleNeedsAnswer(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	sessionActive := existing != nil && existing.SessionActive

	att := &WorktreeState{
		NeedsInput:       true,
		NotificationType: engine.NotificationElicitationDialog,
		SessionActive:    sessionActive,
		UpdatedAt:        event.Timestamp,
	}
	s.attention[key] = att

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, att, s.terminals[key])}
}

// handleSessionStart marks the worktree as having an active Claude session
// and clears any stale attention state.
func (s *Store) handleSessionStart(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	isAlreadyActive := existing != nil && existing.SessionActive && !existing.NeedsInput
	if isAlreadyActive {
		return nil // No change — skip broadcast.
	}

	state := &WorktreeState{SessionActive: true, UpdatedAt: event.Timestamp}
	s.attention[key] = state

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, state, s.terminals[key])}
}

// handleSessionEnd marks the worktree's Claude session as ended
// and clears attention state.
func (s *Store) handleSessionEnd(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	isAlreadyInactive := existing != nil && !existing.SessionActive && !existing.NeedsInput
	if isAlreadyInactive {
		return nil // No change — skip broadcast.
	}

	state := &WorktreeState{SessionActive: false, UpdatedAt: event.Timestamp}
	s.attention[key] = state

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, state, s.terminals[key])}
}

// handleStop processes a stop event, updating cost data if present.
// Returns the parsed CostData so HandleEvent can pass it to the onStop
// callback without re-parsing the same JSON.
func (s *Store) handleStop(key worktreeKey, event ContainerEvent) ([]pendingBroadcast, CostData) {
	var broadcasts []pendingBroadcast
	var parsed CostData

	if event.Data != nil {
		if err := json.Unmarshal(event.Data, &parsed); err != nil {
			slog.Warn("invalid cost data", "err", err, "container", event.ContainerName)
		} else if parsed.TotalCost > 0 {
			cost := &ProjectCost{
				TotalCost:    parsed.TotalCost,
				MessageCount: parsed.MessageCount,
				IsEstimated:  parsed.IsEstimated,
				UpdatedAt:    event.Timestamp,
			}
			s.costs[event.ContainerName] = cost
			broadcasts = append(broadcasts, projectBroadcast(event.ProjectID, event.ContainerName, cost))
		}
	}

	// Clear attention — Claude just responded, so it's working.
	existing, ok := s.attention[key]
	if ok && existing.NeedsInput {
		att := &WorktreeState{
			SessionActive: existing.SessionActive,
			UpdatedAt:     event.Timestamp,
		}
		s.attention[key] = att
		broadcasts = append(broadcasts, buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, att, s.terminals[key]))
	}

	return broadcasts, parsed
}

// handleTerminalConnected sets terminal state when an abduco session starts.
func (s *Store) handleTerminalConnected(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	ts := &TerminalState{
		AbducoAlive:     true,
		ViewerConnected: true,
		ExitCode:        -1,
		UpdatedAt:       event.Timestamp,
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, s.attention[key], ts)}
}

// handleTerminalDisconnected marks the viewer as disconnected.
// The abduco session continues running in the background.
func (s *Store) handleTerminalDisconnected(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.terminals[key]

	ts := &TerminalState{
		AbducoAlive: true,
		ExitCode:    -1,
		UpdatedAt:   event.Timestamp,
	}
	// Preserve abduco and exit code from existing state if available.
	if existing != nil {
		ts.AbducoAlive = existing.AbducoAlive
		ts.ExitCode = existing.ExitCode
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, s.attention[key], ts)}
}

// handleProcessKilled marks both ttyd and abduco as dead.
func (s *Store) handleProcessKilled(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	ts := &TerminalState{
		ExitCode:  -1,
		UpdatedAt: event.Timestamp,
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, s.attention[key], ts)}
}

// handleSessionExit records Claude's exit code.
// session_exit fires inside the running abduco session, so if we have no
// prior terminal state the terminal must have been connected.
func (s *Store) handleSessionExit(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	var data SessionExitData
	if event.Data != nil {
		if err := json.Unmarshal(event.Data, &data); err != nil {
			slog.Warn("invalid session_exit data", "err", err, "container", event.ContainerName)
		}
	}

	existing := s.terminals[key]
	ts := &TerminalState{
		AbducoAlive:     true,
		ViewerConnected: true,
		ExitCode:        data.ExitCode,
		UpdatedAt:       event.Timestamp,
	}
	if existing != nil {
		ts.AbducoAlive = existing.AbducoAlive
		ts.ViewerConnected = existing.ViewerConnected
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.ProjectID, event.ContainerName, event.WorktreeID, s.attention[key], ts)}
}

// GetTerminalState returns the terminal lifecycle state for a worktree.
// Returns zero value if no terminal state exists.
func (s *Store) GetTerminalState(containerName, worktreeID string) TerminalState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := worktreeKey{containerName: containerName, worktreeID: worktreeID}
	if ts, ok := s.terminals[key]; ok {
		return *ts
	}
	return TerminalState{}
}

// EvictWorktree removes all cached state (attention + terminal) for a single
// worktree. Called after a worktree is removed or cleaned up so the UI stops
// showing stale entries. Broadcasts a cleared state event so connected
// frontends update immediately.
func (s *Store) EvictWorktree(containerName, worktreeID string) {
	s.mu.Lock()

	key := worktreeKey{containerName: containerName, worktreeID: worktreeID}
	delete(s.attention, key)
	delete(s.terminals, key)

	// Rebuild terminalContainers: if no terminals remain for this container,
	// remove the secondary index entry so HasTerminalData returns false.
	hasRemaining := false
	for k := range s.terminals {
		if k.containerName == containerName {
			hasRemaining = true
			break
		}
	}
	if !hasRemaining {
		delete(s.terminalContainers, containerName)
	}

	s.mu.Unlock()

	// Broadcast cleared state so frontends drop the worktree immediately.
	// TODO(Phase 5): pass projectID through EvictWorktree so SSE events carry it.
	s.broadcast([]pendingBroadcast{buildWorktreeBroadcast("", containerName, worktreeID, nil, nil)})
}

// HasTerminalData reports whether the store has any terminal lifecycle
// entries for the given container. O(1) lookup via secondary index.
func (s *Store) HasTerminalData(containerName string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	_, ok := s.terminalContainers[containerName]
	return ok
}

// ActiveContainers returns the names of all containers that have sent
// at least one event (i.e. have an entry in lastEvents).
func (s *Store) ActiveContainers() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	names := make([]string, 0, len(s.lastEvents))
	for name := range s.lastEvents {
		names = append(names, name)
	}
	return names
}

// MarkContainerStale clears all worktree states for a container that
// has stopped sending heartbeats, and broadcasts updates to frontends.
func (s *Store) MarkContainerStale(containerName string) {
	s.mu.Lock()

	now := time.Now()
	var broadcasts []pendingBroadcast
	broadcastedKeys := make(map[worktreeKey]struct{})

	// Broadcast cleared state for any active worktrees before deleting.
	for key, att := range s.attention {
		if key.containerName != containerName {
			continue
		}
		if !att.SessionActive && !att.NeedsInput {
			continue
		}

		cleared := &WorktreeState{UpdatedAt: now}
		ts := s.terminals[key]
		broadcasts = append(broadcasts, buildWorktreeBroadcast("", containerName, key.worktreeID, cleared, ts))
		broadcastedKeys[key] = struct{}{}
	}

	for key := range s.terminals {
		if key.containerName != containerName {
			continue
		}
		if _, alreadySent := broadcastedKeys[key]; !alreadySent {
			broadcasts = append(broadcasts, buildWorktreeBroadcast("", containerName, key.worktreeID, nil, nil))
		}
	}

	// Remove all state for this container. A new heartbeat or event
	// will re-register it if the container comes back.
	for key := range s.attention {
		if key.containerName == containerName {
			delete(s.attention, key)
		}
	}
	for key := range s.terminals {
		if key.containerName == containerName {
			delete(s.terminals, key)
		}
	}
	delete(s.costs, containerName)
	delete(s.terminalContainers, containerName)
	delete(s.lastEvents, containerName)

	s.mu.Unlock()
	s.broadcast(broadcasts)
}

// BroadcastWorktreeListChanged sends a worktree_list_changed event to all
// SSE clients so they can refresh the worktree list for the given container.
func (s *Store) BroadcastWorktreeListChanged(containerName string) {
	s.broadcast([]pendingBroadcast{{
		event: SSEWorktreeListChanged,
		data: struct {
			ContainerName string `json:"containerName"`
		}{
			ContainerName: containerName,
		},
	}})
}

// broadcastBudgetEvent sends a budget enforcement SSE event with the shared
// [BudgetEventPayload] to all connected frontends.
func (s *Store) broadcastBudgetEvent(event SSEEventType, projectID, containerName string, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: event,
		data: BudgetEventPayload{
			ProjectID:     projectID,
			ContainerName: containerName,
			TotalCost:     totalCost,
			Budget:        budget,
		},
	}})
}

// BroadcastBudgetExceeded sends a budget_exceeded SSE event to all
// connected frontends so they can show a notification.
func (s *Store) BroadcastBudgetExceeded(projectID, containerName string, totalCost, budget float64) {
	s.broadcastBudgetEvent(SSEBudgetExceeded, projectID, containerName, totalCost, budget)
}

// BroadcastBudgetContainerStopped sends a budget_container_stopped SSE event
// after a container is stopped due to budget enforcement, so frontends can
// redirect users away from the now-stopped project.
func (s *Store) BroadcastBudgetContainerStopped(projectID, containerName, containerID string, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: SSEBudgetContainerStopped,
		data: BudgetContainerStoppedPayload{
			BudgetEventPayload: BudgetEventPayload{
				ProjectID:     projectID,
				ContainerName: containerName,
				TotalCost:     totalCost,
				Budget:        budget,
			},
			ContainerID: containerID,
		},
	}})
}

// buildWorktreeBroadcast creates a pending broadcast for a worktree state change,
// including both attention and terminal lifecycle data when available.
func buildWorktreeBroadcast(projectID, containerName, worktreeID string, att *WorktreeState, ts *TerminalState) pendingBroadcast {
	payload := WorktreeStatePayload{
		ProjectID:     projectID,
		ContainerName: containerName,
		WorktreeID:    worktreeID,
	}

	if att != nil {
		payload.NeedsInput = att.NeedsInput
		payload.NotificationType = att.NotificationType
		payload.SessionActive = att.SessionActive
	}

	if ts != nil {
		payload.State = ts.DeriveWorktreeState()
		payload.ExitCode = ts.ExitCode
	}

	return pendingBroadcast{event: SSEWorktreeState, data: payload}
}

// projectBroadcast creates a pending broadcast for a project cost change.
// ProjectStatePayload is the JSON shape sent over SSE for project_state events.
type ProjectStatePayload struct {
	ProjectID     string  `json:"projectId,omitempty"`
	ContainerName string  `json:"containerName"`
	TotalCost     float64 `json:"totalCost"`
	MessageCount  int     `json:"messageCount"`
}

func projectBroadcast(projectID, containerName string, cost *ProjectCost) pendingBroadcast {
	return pendingBroadcast{
		event: SSEProjectState,
		data: ProjectStatePayload{
			ProjectID:     projectID,
			ContainerName: containerName,
			TotalCost:     cost.TotalCost,
			MessageCount:  cost.MessageCount,
		},
	}
}

// writeToAuditLog persists a container event to the audit log via the
// AuditWriter. Skips heartbeat events. The writer handles mode filtering.
func (s *Store) writeToAuditLog(writer *db.AuditWriter, event ContainerEvent) {
	if writer == nil || event.Type == EventHeartbeat {
		return
	}

	source := db.SourceAgent
	eventName := string(event.Type)
	var message string

	switch event.Type {
	case EventTerminalConnected, EventTerminalDisconnected, EventProcessKilled, EventSessionExit:
		source = db.SourceContainer
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
	}

	level := db.LevelInfo
	switch event.Type {
	case EventToolUseFailure, EventStopFailure:
		level = db.LevelError
	}

	entry := db.Entry{
		Timestamp:     event.Timestamp,
		Source:        source,
		Level:         level,
		ProjectID:     event.ProjectID,
		ContainerName: event.ContainerName,
		Worktree:      event.WorktreeID,
		Event:         eventName,
		Message:       message,
		Data:          event.Data,
	}

	writer.Write(entry)
}

// broadcast marshals and sends pending broadcasts to SSE clients.
// Called outside the store lock.
func (s *Store) broadcast(broadcasts []pendingBroadcast) {
	if s.broker == nil || len(broadcasts) == 0 {
		return
	}

	for _, b := range broadcasts {
		data, err := json.Marshal(b.data)
		if err != nil {
			slog.Warn("failed to marshal broadcast", "err", err)
			continue
		}
		s.broker.Broadcast(SSEEvent{Event: b.event, Data: data})
	}
}

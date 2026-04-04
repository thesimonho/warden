package eventbus

import (
	"encoding/hex"
	"encoding/json"
	"hash/fnv"
	"log/slog"
	"strconv"
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
	// SessionAlive is true when the tmux session is running.
	SessionAlive bool
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
	if ts.SessionAlive {
		if !ts.ViewerConnected {
			return engine.WorktreeStateBackground
		}
		if ts.ExitCode >= 0 {
			return engine.WorktreeStateShell
		}
		return engine.WorktreeStateConnected
	}
	return engine.WorktreeStateStopped
}

// WorktreeStatePayload is the JSON shape sent over SSE for worktree_state events.
// Shared by all broadcast helpers to keep the Go and TypeScript types in sync.
type WorktreeStatePayload struct {
	ProjectRef
	WorktreeID       string                  `json:"worktreeId"`
	NeedsInput       bool                    `json:"needsInput"`
	NotificationType engine.NotificationType `json:"notificationType,omitempty"`
	SessionActive    bool                    `json:"sessionActive"`
	State            engine.WorktreeState    `json:"state,omitempty"`
	ExitCode         int                     `json:"exitCode,omitempty"`
}

// CostUpdateCallbackFunc is called on every cost update event with
// cumulative cost data parsed from the event payload. sessionID and
// cost are zero when the event carried no cost data. Set via
// [Store.SetCostUpdateCallback].
// projectID is the deterministic project identifier (from WARDEN_PROJECT_ID).
// agentType is the agent type identifier (from WARDEN_AGENT_TYPE).
// containerName is the Docker container name (from WARDEN_CONTAINER_NAME).
type CostUpdateCallbackFunc func(projectID, agentType, containerName, sessionID string, cost float64, isEstimated bool)

// StaleCallbackFunc is called when a container stops sending heartbeats
// and is marked stale. The service layer uses this to write an audit
// entry with full project context (project ID and name). Set via
// [Store.SetStaleCallback].
type StaleCallbackFunc func(containerName string)

// AliveCallbackFunc is called when a container transitions from
// unknown/stale to alive. Fires on the first lifecycle event
// (heartbeat or session_start) for a container not in lastEvents.
// The service layer uses this to reactively start session watchers.
// Set via [Store.SetAliveCallback].
type AliveCallbackFunc func(projectID, agentType, containerName string)

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
	onCostUpdate       CostUpdateCallbackFunc
	onStale            StaleCallbackFunc
	onAlive            AliveCallbackFunc
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

// SetCostUpdateCallback registers a function called on every cost
// update event. The callback receives cumulative cost data (which may
// be zero/empty) and is responsible for both persistence and budget
// enforcement. This is the single integration point between the
// event bus and the service layer's cost/budget system.
func (s *Store) SetCostUpdateCallback(fn CostUpdateCallbackFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onCostUpdate = fn
}

// SetStaleCallback registers a function called when a container's
// heartbeat goes stale. The service layer implements this to write
// an audit entry with full project context.
func (s *Store) SetStaleCallback(fn StaleCallbackFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onStale = fn
}

// SetAliveCallback registers a function called when a container
// transitions from unknown/stale to alive. Only fires on lifecycle
// events (heartbeat or session_start), not on arbitrary events that
// might arrive for a container before it is fully running.
func (s *Store) SetAliveCallback(fn AliveCallbackFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onAlive = fn
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

	_, wasKnown := s.lastEvents[event.ContainerName]
	isLifecycleEvent := event.Type == EventHeartbeat || event.Type == EventSessionStart
	s.lastEvents[event.ContainerName] = event.Timestamp

	var broadcasts []pendingBroadcast
	var costData CostData // populated by handleCostUpdate, reused by onCostUpdate callback

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
	case EventCostUpdate:
		broadcasts, costData = s.handleCostUpdate(key, event)
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
	case EventTurnComplete:
		broadcasts = s.handleTurnComplete(key, event)
	case EventTurnDuration:
		// No state change — turn duration is logged for audit.
	case EventApiMetrics:
		// No state change — API performance metrics are logged for audit.
	case EventPermissionGrant:
		// No state change — permission grants are logged for audit.
	case EventContextCompact:
		// No state change — context compaction is logged for audit.
	case EventSystemInfo:
		// No state change — informational system messages are logged for audit.
	case EventRuntimeInstalling, EventRuntimeInstalled:
		broadcasts = s.handleRuntimeStatus(event)
	case EventAgentInstalling, EventAgentInstalled:
		broadcasts = s.handleAgentStatus(event)
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
	onCostUpdate := s.onCostUpdate
	onAlive := s.onAlive
	s.mu.Unlock()

	// Broadcast first so SSE notifications reach frontends before the
	// audit DB write, which involves SQLite I/O and can add latency.
	s.broadcast(broadcasts)

	// Write to the audit log (outside the lock).
	s.writeToAuditLog(writer, event)

	// On every cost update, invoke the callback for cost persistence and
	// budget enforcement. Uses cost data already parsed by handleCostUpdate.
	// Runs outside the lock because enforcement may call back into docker.
	if event.Type == EventCostUpdate && onCostUpdate != nil {
		onCostUpdate(event.ProjectID, event.AgentType, event.ContainerName, costData.SessionID, costData.TotalCost, costData.IsEstimated)
	}

	// Fire alive callback when a container appears for the first time
	// (or reappears after being marked stale). Only lifecycle events
	// trigger this to avoid spurious watcher starts from stray events.
	if !wasKnown && isLifecycleEvent && onAlive != nil {
		onAlive(event.ProjectID, event.AgentType, event.ContainerName)
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

// GetContainerWorktreeStates returns all worktree attention states for a container.
// Used by the service layer to aggregate attention across worktrees at the project level.
func (s *Store) GetContainerWorktreeStates(containerName string) map[string]WorktreeState {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]WorktreeState)
	for key, att := range s.attention {
		if key.containerName == containerName {
			result[key.worktreeID] = *att
		}
	}
	return result
}

// SeedWorktreeBaseline initializes the attention state for a worktree
// with UpdatedAt set to the current time. This prevents historical JSONL
// events (replayed during session watcher catch-up) from setting stale
// attention state — handleTurnComplete's timestamp check rejects events
// older than UpdatedAt. No-op if state already exists for this worktree.
func (s *Store) SeedWorktreeBaseline(containerName, worktreeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := worktreeKey{containerName: containerName, worktreeID: worktreeID}
	if _, exists := s.attention[key]; exists {
		return
	}
	s.attention[key] = &WorktreeState{UpdatedAt: time.Now()}
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

	return []pendingBroadcast{
		buildWorktreeBroadcast(event.Ref(), event.WorktreeID, att, s.terminals[key]),
		s.buildProjectBroadcast(event.Ref()),
	}
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

	return []pendingBroadcast{
		buildWorktreeBroadcast(event.Ref(), event.WorktreeID, att, s.terminals[key]),
		s.buildProjectBroadcast(event.Ref()),
	}
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

	return []pendingBroadcast{
		buildWorktreeBroadcast(event.Ref(), event.WorktreeID, att, s.terminals[key]),
		s.buildProjectBroadcast(event.Ref()),
	}
}

// handleSessionStart marks the worktree as having an active Claude session
// and clears any stale attention state.
func (s *Store) handleSessionStart(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	isAlreadyActive := existing != nil && existing.SessionActive && !existing.NeedsInput
	if isAlreadyActive {
		return nil // No change — skip broadcast.
	}

	hadAttention := existing != nil && existing.NeedsInput

	// Preserve the more recent UpdatedAt so a seeded baseline (from
	// SeedWorktreeBaseline) isn't overwritten by a historical session_start
	// replayed during JSONL catch-up.
	updatedAt := event.Timestamp
	if existing != nil && existing.UpdatedAt.After(updatedAt) {
		updatedAt = existing.UpdatedAt
	}

	state := &WorktreeState{SessionActive: true, UpdatedAt: updatedAt}
	s.attention[key] = state

	broadcasts := []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, state, s.terminals[key])}
	if hadAttention {
		broadcasts = append(broadcasts, s.buildProjectBroadcast(event.Ref()))
	}
	return broadcasts
}

// handleSessionEnd marks the worktree's Claude session as ended
// and clears attention state.
func (s *Store) handleSessionEnd(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	isAlreadyInactive := existing != nil && !existing.SessionActive && !existing.NeedsInput
	if isAlreadyInactive {
		return nil // No change — skip broadcast.
	}

	hadAttention := existing != nil && existing.NeedsInput
	state := &WorktreeState{SessionActive: false, UpdatedAt: event.Timestamp}
	s.attention[key] = state

	broadcasts := []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, state, s.terminals[key])}
	if hadAttention {
		broadcasts = append(broadcasts, s.buildProjectBroadcast(event.Ref()))
	}
	return broadcasts
}

// handleTurnComplete sets "waiting for input" attention state when an agent
// turn ends. This signals that the agent is idle at the prompt, supplementing
// the real-time Notification hook (which may not fire in all cases, e.g.
// after --continue resume). Only sets attention if the session is active and
// not already in an attention state, and the event is newer than the current
// state (to avoid stale JSONL events overriding fresher hook events).
func (s *Store) handleTurnComplete(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.attention[key]
	if existing == nil || !existing.SessionActive {
		return nil
	}
	if existing.NeedsInput {
		return nil // Already in an attention state — don't downgrade.
	}
	if !existing.UpdatedAt.IsZero() && existing.UpdatedAt.After(event.Timestamp) {
		return nil // Stale turn_complete — a newer event already updated state.
	}

	att := &WorktreeState{
		NeedsInput:       true,
		NotificationType: engine.NotificationIdlePrompt,
		SessionActive:    existing.SessionActive,
		UpdatedAt:        event.Timestamp,
	}
	s.attention[key] = att

	return []pendingBroadcast{
		buildWorktreeBroadcast(event.Ref(), event.WorktreeID, att, s.terminals[key]),
		s.buildProjectBroadcast(event.Ref()),
	}
}

// handleCostUpdate processes a cost update event, updating in-memory cost
// state if present. Returns the parsed CostData so HandleEvent can pass
// it to the onCostUpdate callback without re-parsing the same JSON.
func (s *Store) handleCostUpdate(key worktreeKey, event ContainerEvent) ([]pendingBroadcast, CostData) {
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
			broadcasts = append(broadcasts, s.buildProjectBroadcast(event.Ref()))
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
		broadcasts = append(broadcasts, buildWorktreeBroadcast(event.Ref(), event.WorktreeID, att, s.terminals[key]))
	}

	return broadcasts, parsed
}

// handleRuntimeStatus broadcasts runtime installation progress to SSE clients.
func (s *Store) handleRuntimeStatus(event ContainerEvent) []pendingBroadcast {
	var data RuntimeStatusData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		slog.Warn("malformed runtime status event", "err", err, "container", event.ContainerName)
		return nil
	}

	phase := "installing"
	if event.Type == EventRuntimeInstalled {
		phase = "installed"
	}

	return []pendingBroadcast{{
		event: SSERuntimeStatus,
		data: RuntimeStatusPayload{
			ProjectRef:   event.Ref(),
			Phase:        phase,
			RuntimeID:    data.RuntimeID,
			RuntimeLabel: data.RuntimeLabel,
		},
	}}
}

// handleAgentStatus broadcasts agent CLI installation progress to SSE clients.
func (s *Store) handleAgentStatus(event ContainerEvent) []pendingBroadcast {
	var data AgentStatusData
	if err := json.Unmarshal(event.Data, &data); err != nil {
		slog.Warn("malformed agent status event", "err", err, "container", event.ContainerName)
		return nil
	}

	phase := "installing"
	if event.Type == EventAgentInstalled {
		phase = "installed"
	}

	return []pendingBroadcast{{
		event: SSEAgentStatus,
		data: AgentStatusPayload{
			ProjectRef: event.Ref(),
			Phase:      phase,
			Version:    data.Version,
		},
	}}
}

// handleTerminalConnected sets terminal state when a tmux session starts.
func (s *Store) handleTerminalConnected(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	ts := &TerminalState{
		SessionAlive:    true,
		ViewerConnected: true,
		ExitCode:        -1,
		UpdatedAt:       event.Timestamp,
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, s.attention[key], ts)}
}

// handleTerminalDisconnected marks the viewer as disconnected.
// The tmux session continues running in the background.
func (s *Store) handleTerminalDisconnected(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	existing := s.terminals[key]

	ts := &TerminalState{
		SessionAlive: true,
		ExitCode:     -1,
		UpdatedAt:    event.Timestamp,
	}
	// Preserve session alive and exit code from existing state if available.
	if existing != nil {
		ts.SessionAlive = existing.SessionAlive
		ts.ExitCode = existing.ExitCode
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, s.attention[key], ts)}
}

// handleProcessKilled marks the tmux session as dead.
func (s *Store) handleProcessKilled(key worktreeKey, event ContainerEvent) []pendingBroadcast {
	ts := &TerminalState{
		ExitCode:  -1,
		UpdatedAt: event.Timestamp,
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, s.attention[key], ts)}
}

// handleSessionExit records Claude's exit code.
// session_exit fires inside the running tmux session, so if we have no
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
		SessionAlive:    true,
		ViewerConnected: true,
		ExitCode:        data.ExitCode,
		UpdatedAt:       event.Timestamp,
	}
	if existing != nil {
		ts.SessionAlive = existing.SessionAlive
		ts.ViewerConnected = existing.ViewerConnected
	}
	s.terminals[key] = ts
	s.terminalContainers[event.ContainerName] = struct{}{}

	return []pendingBroadcast{buildWorktreeBroadcast(event.Ref(), event.WorktreeID, s.attention[key], ts)}
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
	s.broadcast([]pendingBroadcast{buildWorktreeBroadcast(ProjectRef{ContainerName: containerName}, worktreeID, nil, nil)})
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
		broadcasts = append(broadcasts, buildWorktreeBroadcast(ProjectRef{ContainerName: containerName}, key.worktreeID, cleared, ts))
		broadcastedKeys[key] = struct{}{}
	}

	for key := range s.terminals {
		if key.containerName != containerName {
			continue
		}
		if _, alreadySent := broadcastedKeys[key]; !alreadySent {
			broadcasts = append(broadcasts, buildWorktreeBroadcast(ProjectRef{ContainerName: containerName}, key.worktreeID, nil, nil))
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

	onStale := s.onStale
	s.mu.Unlock()

	s.broadcast(broadcasts)

	if onStale != nil {
		onStale(containerName)
	}
}

// RemoveContainer clears all in-memory state for a container without
// triggering the stale callback. Called when a container is deliberately
// deleted — prevents the liveness checker from later finding a stale entry
// for the old container name and inadvertently stopping a newly created
// container's session watcher.
func (s *Store) RemoveContainer(containerName string) {
	s.mu.Lock()
	defer s.mu.Unlock()

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
}

// BroadcastWorktreeListChanged sends a worktree_list_changed event to all
// SSE clients so they can refresh the worktree list for the given container.
func (s *Store) BroadcastWorktreeListChanged(ref ProjectRef) {
	s.broadcast([]pendingBroadcast{{
		event: SSEWorktreeListChanged,
		data:  ref,
	}})
}

// broadcastBudgetEvent sends a budget enforcement SSE event with the shared
// [BudgetEventPayload] to all connected frontends.
func (s *Store) broadcastBudgetEvent(event SSEEventType, ref ProjectRef, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: event,
		data: BudgetEventPayload{
			ProjectRef: ref,
			TotalCost:  totalCost,
			Budget:     budget,
		},
	}})
}

// BroadcastBudgetExceeded sends a budget_exceeded SSE event to all
// connected frontends so they can show a notification.
func (s *Store) BroadcastBudgetExceeded(ref ProjectRef, totalCost, budget float64) {
	s.broadcastBudgetEvent(SSEBudgetExceeded, ref, totalCost, budget)
}

// BroadcastBudgetContainerStopped sends a budget_container_stopped SSE event
// after a container is stopped due to budget enforcement, so frontends can
// redirect users away from the now-stopped project.
func (s *Store) BroadcastBudgetContainerStopped(ref ProjectRef, containerID string, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: SSEBudgetContainerStopped,
		data: BudgetContainerStoppedPayload{
			BudgetEventPayload: BudgetEventPayload{
				ProjectRef: ref,
				TotalCost:  totalCost,
				Budget:     budget,
			},
			ContainerID: containerID,
		},
	}})
}

// buildWorktreeBroadcast creates a pending broadcast for a worktree state change,
// including both attention and terminal lifecycle data when available.
func buildWorktreeBroadcast(ref ProjectRef, worktreeID string, att *WorktreeState, ts *TerminalState) pendingBroadcast {
	payload := WorktreeStatePayload{
		ProjectRef: ref,
		WorktreeID: worktreeID,
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

// ProjectStatePayload is the JSON shape sent over SSE for project_state events.
// Carries both cost and attention state so the home page can update in real time.
type ProjectStatePayload struct {
	ProjectRef
	TotalCost        float64                 `json:"totalCost"`
	MessageCount     int                     `json:"messageCount"`
	NeedsInput       bool                    `json:"needsInput"`
	NotificationType engine.NotificationType `json:"notificationType,omitempty"`
}

// buildProjectBroadcast creates a project_state broadcast with complete state:
// aggregated attention across all worktrees plus current cost. Every project_state
// event carries the full snapshot so the frontend can apply it unconditionally.
// Must be called under lock.
func (s *Store) buildProjectBroadcast(ref ProjectRef) pendingBroadcast {
	needsInput, highestType := s.aggregateContainerAttention(ref.ContainerName)

	payload := ProjectStatePayload{
		ProjectRef:       ref,
		NeedsInput:       needsInput,
		NotificationType: highestType,
	}

	if cost, ok := s.costs[ref.ContainerName]; ok {
		payload.TotalCost = cost.TotalCost
		payload.MessageCount = cost.MessageCount
	}

	return pendingBroadcast{event: SSEProjectState, data: payload}
}

// AggregateContainerAttention returns the highest-priority attention state
// across all worktrees for a container. The internal variant (lowercase) is
// used under existing lock; this public variant acquires its own read lock.
func (s *Store) AggregateContainerAttention(containerName string) (needsInput bool, highest engine.NotificationType) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aggregateContainerAttention(containerName)
}

// aggregateContainerAttention returns the highest-priority attention state
// across all worktrees for a container. Must be called under lock.
func (s *Store) aggregateContainerAttention(containerName string) (needsInput bool, highest engine.NotificationType) {
	for key, att := range s.attention {
		if key.containerName != containerName || !att.NeedsInput {
			continue
		}
		needsInput = true
		if highest == "" || engine.NotificationPriority(att.NotificationType) > engine.NotificationPriority(highest) {
			highest = att.NotificationType
		}
	}
	return
}

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

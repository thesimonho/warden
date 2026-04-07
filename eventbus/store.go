// Package eventbus implements the runtime event processing pipeline.
// It maintains in-memory container state, broadcasts changes to SSE
// subscribers, writes audit logs, and monitors container liveness.
// Event types are defined in the [event] package; filesystem ingestion
// is handled by [watcher/hook].
package eventbus

import (
	"log/slog"
	"sync"
	"time"

	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/event"
)

// WorktreeState holds the real-time state for a single worktree,
// derived from container hook events pushed via the event bus.
type WorktreeState struct {
	// NeedsInput is true when Claude is waiting for user attention.
	NeedsInput bool
	// NotificationType indicates why attention is needed.
	NotificationType event.NotificationType
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
	event event.SSEEventType
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
	event.ProjectRef
	WorktreeID       string                 `json:"worktreeId"`
	NeedsInput       bool                   `json:"needsInput"`
	NotificationType event.NotificationType `json:"notificationType,omitempty"`
	SessionActive    bool                   `json:"sessionActive"`
	State            engine.WorktreeState   `json:"state,omitempty"`
	ExitCode         int                    `json:"exitCode,omitempty"`
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

// costBroadcastMinDelta is the minimum cost change (in USD) required to
// broadcast a project_state SSE event. Suppresses high-frequency broadcasts
// during active agent work when cost changes are negligible.
const costBroadcastMinDelta = 0.01

// costBroadcastMinInterval is the minimum time between cost-triggered
// project_state broadcasts for the same container.
const costBroadcastMinInterval = 5 * time.Second

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

	// Cost broadcast throttling — suppresses redundant SSE events when
	// cost changes are negligible (< $0.01 within 5s).
	lastBroadcastCost map[string]float64
	lastBroadcastTime map[string]time.Time
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
		lastBroadcastCost:  make(map[string]float64),
		lastBroadcastTime:  make(map[string]time.Time),
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
func (s *Store) HandleEvent(evt event.ContainerEvent) {
	s.mu.Lock()

	key := worktreeKey{
		containerName: evt.ContainerName,
		worktreeID:    evt.WorktreeID,
	}

	_, wasKnown := s.lastEvents[evt.ContainerName]
	isLifecycleEvent := evt.Type == event.EventHeartbeat || evt.Type == event.EventSessionStart
	s.lastEvents[evt.ContainerName] = evt.Timestamp

	var broadcasts []pendingBroadcast
	var costData event.CostData // populated by handleCostUpdate, reused by onCostUpdate callback

	switch evt.Type {
	case event.EventAttention:
		broadcasts = s.handleAttention(key, evt)
	case event.EventAttentionClear:
		broadcasts = s.handleAttentionClear(key, evt)
	case event.EventNeedsAnswer:
		broadcasts = s.handleNeedsAnswer(key, evt)
	case event.EventSessionStart:
		broadcasts = s.handleSessionStart(key, evt)
	case event.EventSessionEnd:
		broadcasts = s.handleSessionEnd(key, evt)
	case event.EventCostUpdate:
		broadcasts, costData = s.handleCostUpdate(key, evt)
	case event.EventHeartbeat:
		// No-op — lastEvents is already updated above for all event types.
	case event.EventUserPrompt:
		// No state change — the prompt is logged to the audit log by writeToAuditLog below.
	case event.EventToolUse, event.EventToolUseFailure:
		// No state change — tool events are logged to the audit log by writeToAuditLog below.
	case event.EventStopFailure:
		// No state change — stop failure is logged for audit.
	case event.EventPermissionRequest:
		// No state change — permission requests are logged for audit.
	case event.EventSubagentStart, event.EventSubagentStop:
		// No state change — subagent lifecycle is logged for audit.
	case event.EventConfigChange, event.EventInstructionsLoaded:
		// No state change — config events are logged for audit.
	case event.EventTaskCompleted:
		// No state change — task completion is logged for audit.
	case event.EventElicitation, event.EventElicitationResult:
		// No state change — MCP elicitation events are logged for audit.
	case event.EventTurnComplete:
		broadcasts = s.handleTurnComplete(key, evt)
	case event.EventTurnDuration:
		// No state change — turn duration is logged for audit.
	case event.EventApiMetrics:
		// No state change — API performance metrics are logged for audit.
	case event.EventPermissionGrant:
		// No state change — permission grants are logged for audit.
	case event.EventContextCompact:
		// No state change — context compaction is logged for audit.
	case event.EventSystemInfo:
		// No state change — informational system messages are logged for audit.
	case event.EventRuntimeInstalling, event.EventRuntimeInstalled:
		broadcasts = s.handleRuntimeStatus(evt)
	case event.EventAgentInstalling, event.EventAgentInstalled:
		broadcasts = s.handleAgentStatus(evt)
	case event.EventNetworkBlocked:
		// No state change — blocked connections are logged for audit.
	case event.EventContainerError:
		// No state change — fatal container errors are logged for audit.
	case event.EventTerminalConnected:
		broadcasts = s.handleTerminalConnected(key, evt)
	case event.EventTerminalDisconnected:
		broadcasts = s.handleTerminalDisconnected(key, evt)
	case event.EventProcessKilled:
		broadcasts = s.handleProcessKilled(key, evt)
	case event.EventSessionExit:
		broadcasts = s.handleSessionExit(key, evt)
	default:
		slog.Warn("unknown event type", "type", evt.Type, "container", evt.ContainerName)
	}

	writer := s.auditWriter
	onCostUpdate := s.onCostUpdate
	onAlive := s.onAlive
	s.mu.Unlock()

	// Broadcast first so SSE notifications reach frontends before the
	// audit DB write, which involves SQLite I/O and can add latency.
	s.broadcast(broadcasts)

	// Write to the audit log (outside the lock).
	s.writeToAuditLog(writer, evt)

	// On every cost update, invoke the callback for cost persistence and
	// budget enforcement. Uses cost data already parsed by handleCostUpdate.
	// Runs outside the lock because enforcement may call back into docker.
	if evt.Type == event.EventCostUpdate && onCostUpdate != nil {
		onCostUpdate(evt.ProjectID, evt.AgentType, evt.ContainerName, costData.SessionID, costData.TotalCost, costData.IsEstimated)
	}

	// Fire alive callback when a container appears for the first time
	// (or reappears after being marked stale). Only lifecycle events
	// trigger this to avoid spurious watcher starts from stray events.
	if !wasKnown && isLifecycleEvent && onAlive != nil {
		onAlive(evt.ProjectID, evt.AgentType, evt.ContainerName)
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
		// Prune cost broadcast throttle state for this container since
		// no terminals remain.
		delete(s.lastBroadcastCost, containerName)
		delete(s.lastBroadcastTime, containerName)
	}

	s.mu.Unlock()

	// Broadcast cleared state so frontends drop the worktree immediately.
	// TODO(Phase 5): pass projectID through EvictWorktree so SSE events carry it.
	s.broadcast([]pendingBroadcast{buildWorktreeBroadcast(event.ProjectRef{ContainerName: containerName}, worktreeID, nil, nil)})
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

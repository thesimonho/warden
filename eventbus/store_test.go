package eventbus

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/thesimonho/warden/engine"
)

func mustMarshal(t *testing.T, v any) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestStore_HandleAttention(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-1", "main")
	if !att.NeedsInput {
		t.Error("expected NeedsInput to be true")
	}
	if att.NotificationType != engine.NotificationPermissionPrompt {
		t.Errorf("expected permission_prompt, got %q", att.NotificationType)
	}
}

func TestStore_HandleAttentionClear(t *testing.T) {
	store := NewStore(nil, nil)

	// Set attention first.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	// Clear it.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttentionClear,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-1", "main")
	if att.NeedsInput {
		t.Error("expected NeedsInput to be false after clear")
	}
}

func TestStore_HandleNeedsAnswer(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventNeedsAnswer,
		ContainerName: "proj-1",
		WorktreeID:    "feature-x",
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-1", "feature-x")
	if !att.NeedsInput {
		t.Error("expected NeedsInput to be true")
	}
	if att.NotificationType != engine.NotificationElicitationDialog {
		t.Errorf("expected elicitation_dialog, got %q", att.NotificationType)
	}
}

func TestStore_HandleSessionStart_SetsActiveAndClearsAttention(t *testing.T) {
	store := NewStore(nil, nil)

	// Set stale attention.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	// New session starts.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	ws := store.GetWorktreeState("proj-1", "main")
	if ws.NeedsInput {
		t.Error("expected NeedsInput to be false after session start")
	}
	if !ws.SessionActive {
		t.Error("expected SessionActive to be true after session start")
	}
}

func TestStore_HandleSessionEnd_ClearsActiveAndAttention(t *testing.T) {
	store := NewStore(nil, nil)

	// Start session first.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Set attention during session.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	// Session ends.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionEnd,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	ws := store.GetWorktreeState("proj-1", "main")
	if ws.NeedsInput {
		t.Error("expected NeedsInput to be false after session end")
	}
	if ws.SessionActive {
		t.Error("expected SessionActive to be false after session end")
	}
}

func TestStore_SessionActive_PreservedAcrossAttentionEvents(t *testing.T) {
	store := NewStore(nil, nil)

	// Start session.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Attention event during active session.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	ws := store.GetWorktreeState("proj-1", "main")
	if !ws.SessionActive {
		t.Error("expected SessionActive to be preserved after attention event")
	}
	if !ws.NeedsInput {
		t.Error("expected NeedsInput to be true")
	}

	// Clear attention.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttentionClear,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	ws = store.GetWorktreeState("proj-1", "main")
	if !ws.SessionActive {
		t.Error("expected SessionActive to be preserved after attention clear")
	}
	if ws.NeedsInput {
		t.Error("expected NeedsInput to be false after clear")
	}
}

func TestStore_HandleStop_UpdatesCost(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventStop,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 1.50, MessageCount: 42}),
		Timestamp:     time.Now(),
	})

	cost := store.GetProjectCost("proj-1")
	if cost.TotalCost != 1.50 {
		t.Errorf("expected totalCost 1.50, got %f", cost.TotalCost)
	}
	if cost.MessageCount != 42 {
		t.Errorf("expected messageCount 42, got %d", cost.MessageCount)
	}
}

func TestStore_HandleStop_ClearsAttention(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventStop,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 0.5, MessageCount: 1}),
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-1", "main")
	if att.NeedsInput {
		t.Error("expected NeedsInput to be false after stop")
	}
}

func TestStore_LastEventTime(t *testing.T) {
	store := NewStore(nil, nil)

	before := store.LastEventTime("proj-1")
	if !before.IsZero() {
		t.Error("expected zero time for unknown container")
	}

	now := time.Now()
	store.HandleEvent(ContainerEvent{
		Type:          EventAttentionClear,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     now,
	})

	after := store.LastEventTime("proj-1")
	if !after.Equal(now) {
		t.Errorf("expected %v, got %v", now, after)
	}
}

func TestStore_GetWorktreeState_Unknown(t *testing.T) {
	store := NewStore(nil, nil)

	att := store.GetWorktreeState("nonexistent", "main")
	if att.NeedsInput {
		t.Error("expected false for unknown worktree")
	}
}

func TestStore_GetProjectCost_Unknown(t *testing.T) {
	store := NewStore(nil, nil)

	cost := store.GetProjectCost("nonexistent")
	if cost.TotalCost != 0 {
		t.Errorf("expected 0, got %f", cost.TotalCost)
	}
}

func TestStore_ConcurrentAccess(t *testing.T) {
	store := NewStore(nil, nil)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(2)
		go func(i int) {
			defer wg.Done()
			store.HandleEvent(ContainerEvent{
				Type:          EventAttention,
				ContainerName: "proj-1",
				WorktreeID:    "main",
				Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
				Timestamp:     time.Now(),
			})
		}(i)
		go func(i int) {
			defer wg.Done()
			_ = store.GetWorktreeState("proj-1", "main")
			_ = store.GetProjectCost("proj-1")
			_ = store.LastEventTime("proj-1")
		}(i)
	}
	wg.Wait()
}

func TestStore_IsolatesContainers(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-2", "main")
	if att.NeedsInput {
		t.Error("container isolation broken: proj-2 should not see proj-1 attention")
	}
}

func TestStore_IsolatesWorktrees(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "feature-a",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	att := store.GetWorktreeState("proj-1", "feature-b")
	if att.NeedsInput {
		t.Error("worktree isolation broken: feature-b should not see feature-a attention")
	}
}

// ---------------------------------------------------------------------------
// Terminal lifecycle events
// ---------------------------------------------------------------------------

func TestStore_HandleTerminalConnected(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	ts := store.GetTerminalState("proj-1", "main")
	if !ts.ViewerConnected {
		t.Error("expected ViewerConnected true")
	}
	if !ts.AbducoAlive {
		t.Error("expected AbducoAlive true")
	}
	if ts.ExitCode != -1 {
		t.Errorf("expected ExitCode -1, got %d", ts.ExitCode)
	}
}

func TestStore_HandleTerminalDisconnected(t *testing.T) {
	store := NewStore(nil, nil)

	// Connect first.
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	// Disconnect.
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalDisconnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	ts := store.GetTerminalState("proj-1", "main")
	if ts.ViewerConnected {
		t.Error("expected ViewerConnected false after disconnect")
	}
	if !ts.AbducoAlive {
		t.Error("expected AbducoAlive true (abduco survives disconnect)")
	}
}

func TestStore_HandleProcessKilled(t *testing.T) {
	store := NewStore(nil, nil)

	// Connect first.
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	// Kill everything.
	store.HandleEvent(ContainerEvent{
		Type:          EventProcessKilled,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	ts := store.GetTerminalState("proj-1", "main")
	if ts.ViewerConnected {
		t.Error("expected ViewerConnected false after kill")
	}
	if ts.AbducoAlive {
		t.Error("expected AbducoAlive false after kill")
	}
}

func TestStore_HandleSessionExit(t *testing.T) {
	store := NewStore(nil, nil)

	// Connect first.
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	// Claude exits with code 0.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionExit,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, SessionExitData{ExitCode: 0}),
		Timestamp:     time.Now(),
	})

	ts := store.GetTerminalState("proj-1", "main")
	if ts.ExitCode != 0 {
		t.Errorf("expected ExitCode 0, got %d", ts.ExitCode)
	}
	if !ts.ViewerConnected {
		t.Error("expected ViewerConnected true (viewer still connected after Claude exits)")
	}
}

func TestStore_HandleHeartbeat_UpdatesLastEventTime(t *testing.T) {
	store := NewStore(nil, nil)

	now := time.Now()
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		WorktreeID:    "",
		Timestamp:     now,
	})

	lastEvent := store.LastEventTime("proj-1")
	if !lastEvent.Equal(now) {
		t.Errorf("expected last event time %v, got %v", now, lastEvent)
	}
}

func TestStore_GetTerminalState_Unknown(t *testing.T) {
	store := NewStore(nil, nil)

	ts := store.GetTerminalState("nonexistent", "main")
	if !ts.UpdatedAt.IsZero() {
		t.Error("expected zero UpdatedAt for unknown terminal")
	}
}

func TestStore_HasTerminalData(t *testing.T) {
	store := NewStore(nil, nil)

	if store.HasTerminalData("proj-1") {
		t.Error("expected false for unknown container")
	}

	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	if !store.HasTerminalData("proj-1") {
		t.Error("expected true after terminal connected")
	}

	if store.HasTerminalData("proj-2") {
		t.Error("expected false for different container")
	}
}

func TestStore_ActiveContainers(t *testing.T) {
	store := NewStore(nil, nil)

	containers := store.ActiveContainers()
	if len(containers) != 0 {
		t.Errorf("expected empty, got %v", containers)
	}

	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-2",
		Timestamp:     time.Now(),
	})

	containers = store.ActiveContainers()
	if len(containers) != 2 {
		t.Errorf("expected 2 containers, got %d", len(containers))
	}
}

func TestStore_MarkContainerStale(t *testing.T) {
	store := NewStore(nil, nil)

	// Set up active worktree with terminal.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, TerminalConnectedData{}),
		Timestamp:     time.Now(),
	})

	// Mark stale.
	store.MarkContainerStale("proj-1")

	ws := store.GetWorktreeState("proj-1", "main")
	if ws.SessionActive {
		t.Error("expected SessionActive false after stale")
	}
	if ws.NeedsInput {
		t.Error("expected NeedsInput false after stale")
	}

	ts := store.GetTerminalState("proj-1", "main")
	if ts.ViewerConnected {
		t.Error("expected ViewerConnected false after stale")
	}
	if ts.AbducoAlive {
		t.Error("expected AbducoAlive false after stale")
	}

	// Container should be fully removed from all tracking maps.
	active := store.ActiveContainers()
	for _, name := range active {
		if name == "proj-1" {
			t.Error("stale container should be removed from ActiveContainers")
		}
	}
	if store.HasTerminalData("proj-1") {
		t.Error("stale container should be removed from terminal data")
	}
	if store.LastEventTime("proj-1") != (time.Time{}) {
		t.Error("stale container should have zero LastEventTime")
	}
}

func TestStore_MarkContainerStale_IsolatesContainers(t *testing.T) {
	store := NewStore(nil, nil)

	// Set up two containers.
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-2",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Mark only proj-1 stale.
	store.MarkContainerStale("proj-1")

	ws1 := store.GetWorktreeState("proj-1", "main")
	if ws1.SessionActive {
		t.Error("proj-1 should be stale")
	}

	ws2 := store.GetWorktreeState("proj-2", "main")
	if !ws2.SessionActive {
		t.Error("proj-2 should not be affected")
	}
}

// ---------------------------------------------------------------------------
// DeriveWorktreeState
// ---------------------------------------------------------------------------

func TestDeriveWorktreeState(t *testing.T) {
	tests := []struct {
		name     string
		ts       *TerminalState
		expected engine.WorktreeState
	}{
		{
			name:     "nil terminal state",
			ts:       nil,
			expected: "",
		},
		{
			name:     "zero UpdatedAt",
			ts:       &TerminalState{},
			expected: "",
		},
		{
			name: "viewer connected and abduco alive, no exit code",
			ts: &TerminalState{
				ViewerConnected: true,
				AbducoAlive:     true,
				ExitCode:        -1,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateConnected,
		},
		{
			name: "viewer connected and abduco alive, Claude exited",
			ts: &TerminalState{
				ViewerConnected: true,
				AbducoAlive:     true,
				ExitCode:        0,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateShell,
		},
		{
			name: "only abduco alive (background)",
			ts: &TerminalState{
				ViewerConnected: false,
				AbducoAlive:     true,
				ExitCode:        -1,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateBackground,
		},
		{
			name: "both dead",
			ts: &TerminalState{
				ViewerConnected: false,
				AbducoAlive:     false,
				ExitCode:        -1,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateDisconnected,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.ts.DeriveWorktreeState()
			if got != tc.expected {
				t.Errorf("DeriveWorktreeState() = %q, want %q", got, tc.expected)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Broadcast payload (WorktreeStatePayload)
// ---------------------------------------------------------------------------

func TestBuildWorktreeBroadcast_IncludesTerminalState(t *testing.T) {
	att := &WorktreeState{
		NeedsInput:       true,
		NotificationType: engine.NotificationPermissionPrompt,
		SessionActive:    true,
	}
	ts := &TerminalState{
		ViewerConnected: true,
		AbducoAlive:     true,
		ExitCode:        -1,
		UpdatedAt:       time.Now(),
	}

	b := buildWorktreeBroadcast("project-1", "proj-1", "main", att, ts)

	payload, ok := b.data.(WorktreeStatePayload)
	if !ok {
		t.Fatal("expected WorktreeStatePayload")
	}
	if payload.ContainerName != "proj-1" {
		t.Errorf("expected proj-1, got %s", payload.ContainerName)
	}
	if payload.State != engine.WorktreeStateConnected {
		t.Errorf("expected connected, got %s", payload.State)
	}
	if !payload.NeedsInput {
		t.Error("expected NeedsInput true")
	}
}

func TestStore_EvictWorktree_ClearsAllState(t *testing.T) {
	store := NewStore(nil, nil)

	// Populate attention, terminal, and container tracking state.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "wt-1",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "wt-1",
		Timestamp:     time.Now(),
	})

	// Verify state exists before eviction.
	att := store.GetWorktreeState("proj-1", "wt-1")
	if !att.NeedsInput {
		t.Fatal("precondition: expected attention state before eviction")
	}
	ts := store.GetTerminalState("proj-1", "wt-1")
	if !ts.AbducoAlive {
		t.Fatal("precondition: expected terminal state before eviction")
	}

	// Evict.
	store.EvictWorktree("proj-1", "wt-1")

	// Verify all state is gone.
	att = store.GetWorktreeState("proj-1", "wt-1")
	if att.NeedsInput || !att.UpdatedAt.IsZero() {
		t.Error("expected attention state to be cleared after eviction")
	}
	ts = store.GetTerminalState("proj-1", "wt-1")
	if ts.AbducoAlive || !ts.UpdatedAt.IsZero() {
		t.Error("expected terminal state to be cleared after eviction")
	}
}

func TestStore_EvictWorktree_PreservesOtherWorktrees(t *testing.T) {
	store := NewStore(nil, nil)

	// Populate state for two worktrees.
	for _, wt := range []string{"wt-1", "wt-2"} {
		store.HandleEvent(ContainerEvent{
			Type:          EventTerminalConnected,
			ContainerName: "proj-1",
			WorktreeID:    wt,
			Timestamp:     time.Now(),
		})
	}

	// Evict only wt-1.
	store.EvictWorktree("proj-1", "wt-1")

	// wt-2 should be unaffected.
	ts := store.GetTerminalState("proj-1", "wt-2")
	if !ts.AbducoAlive {
		t.Error("expected wt-2 terminal state to be preserved")
	}

	// HasTerminalData should still be true (wt-2 remains).
	if !store.HasTerminalData("proj-1") {
		t.Error("expected HasTerminalData to remain true with wt-2 still present")
	}
}

func TestStore_EvictWorktree_ClearsTerminalContainersWhenEmpty(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventTerminalConnected,
		ContainerName: "proj-1",
		WorktreeID:    "wt-1",
		Timestamp:     time.Now(),
	})

	store.EvictWorktree("proj-1", "wt-1")

	// No more terminals for this container — HasTerminalData should be false.
	if store.HasTerminalData("proj-1") {
		t.Error("expected HasTerminalData to be false after evicting last worktree")
	}
}

func TestBuildWorktreeBroadcast_NilTerminalState(t *testing.T) {
	att := &WorktreeState{SessionActive: true}

	b := buildWorktreeBroadcast("project-1", "proj-1", "main", att, nil)

	payload, ok := b.data.(WorktreeStatePayload)
	if !ok {
		t.Fatal("expected WorktreeStatePayload")
	}
	if payload.State != "" {
		t.Errorf("expected empty state without terminal data, got %s", payload.State)
	}
	if !payload.SessionActive {
		t.Error("expected SessionActive from attention state")
	}
}

func TestStore_HandleStop_CallbackWithValidCost(t *testing.T) {
	var called struct {
		containerName string
		sessionID     string
		cost          float64
	}

	store := NewStore(nil, nil)
	store.SetStopCallback(func(projectID, containerName, sessionID string, cost float64, isEstimated bool) {
		called.containerName = containerName
		called.sessionID = sessionID
		called.cost = cost
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventStop,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 1.50, SessionID: "sess-1", MessageCount: 1}),
		Timestamp:     time.Now(),
	})

	if called.containerName != "proj-1" {
		t.Errorf("expected container proj-1, got %q", called.containerName)
	}
	if called.sessionID != "sess-1" {
		t.Errorf("expected sessionID sess-1, got %q", called.sessionID)
	}
	if called.cost != 1.50 {
		t.Errorf("expected cost 1.50, got %f", called.cost)
	}
}

func TestStore_HandleStop_CallbackWithZeroCost(t *testing.T) {
	var called bool
	var receivedCost float64

	store := NewStore(nil, nil)
	store.SetStopCallback(func(projectID, containerName, sessionID string, cost float64, isEstimated bool) {
		called = true
		receivedCost = cost
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventStop,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 0, MessageCount: 0}),
		Timestamp:     time.Now(),
	})

	if !called {
		t.Error("stop callback must be called even with zero cost")
	}
	if receivedCost != 0 {
		t.Errorf("expected cost 0, got %f", receivedCost)
	}
}

func TestStore_HandleStop_CallbackWithNilData(t *testing.T) {
	var called bool

	store := NewStore(nil, nil)
	store.SetStopCallback(func(projectID, containerName, sessionID string, cost float64, isEstimated bool) {
		called = true
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventStop,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          nil,
		Timestamp:     time.Now(),
	})

	if !called {
		t.Error("stop callback must be called even with nil data")
	}
}

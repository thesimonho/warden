package eventbus

import (
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/thesimonho/warden/db"
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

func TestStore_HandleCostUpdate_UpdatesCost(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
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

func TestStore_HandleCostUpdate_ClearsAttention(t *testing.T) {
	store := NewStore(nil, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
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
	if !ts.SessionAlive {
		t.Error("expected SessionAlive true")
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
	if !ts.SessionAlive {
		t.Error("expected SessionAlive true (session survives disconnect)")
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
	if ts.SessionAlive {
		t.Error("expected SessionAlive false after kill")
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
	if ts.SessionAlive {
		t.Error("expected SessionAlive false after stale")
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
			name: "viewer connected and session alive, no exit code",
			ts: &TerminalState{
				ViewerConnected: true,
				SessionAlive:     true,
				ExitCode:        -1,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateConnected,
		},
		{
			name: "viewer connected and session alive, Claude exited",
			ts: &TerminalState{
				ViewerConnected: true,
				SessionAlive:     true,
				ExitCode:        0,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateShell,
		},
		{
			name: "only session alive (background)",
			ts: &TerminalState{
				ViewerConnected: false,
				SessionAlive:     true,
				ExitCode:        -1,
				UpdatedAt:       time.Now(),
			},
			expected: engine.WorktreeStateBackground,
		},
		{
			name: "both dead",
			ts: &TerminalState{
				ViewerConnected: false,
				SessionAlive:     false,
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
		SessionAlive:     true,
		ExitCode:        -1,
		UpdatedAt:       time.Now(),
	}

	b := buildWorktreeBroadcast(ProjectRef{ProjectID: "project-1", AgentType: "claude-code", ContainerName: "proj-1"}, "main", att, ts)

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

func TestStore_GetContainerWorktreeStates(t *testing.T) {
	store := NewStore(nil, nil)

	// Two worktrees in proj-1, one in proj-2.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		WorktreeID:    "wt-1",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "wt-2",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-2",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	states := store.GetContainerWorktreeStates("proj-1")
	if len(states) != 2 {
		t.Fatalf("expected 2 worktree states for proj-1, got %d", len(states))
	}
	if !states["wt-1"].NeedsInput {
		t.Error("expected wt-1 NeedsInput=true")
	}
	if states["wt-2"].NeedsInput {
		t.Error("expected wt-2 NeedsInput=false")
	}

	// proj-2 should not leak into proj-1.
	if _, ok := states["main"]; ok {
		t.Error("proj-2 worktree leaked into proj-1 states")
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
	if !ts.SessionAlive {
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
	if ts.SessionAlive || !ts.UpdatedAt.IsZero() {
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
	if !ts.SessionAlive {
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

	b := buildWorktreeBroadcast(ProjectRef{ProjectID: "project-1", AgentType: "claude-code", ContainerName: "proj-1"}, "main", att, nil)

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

func TestStore_HandleCostUpdate_CallbackWithValidCost(t *testing.T) {
	var called struct {
		containerName string
		sessionID     string
		cost          float64
	}

	store := NewStore(nil, nil)
	store.SetCostUpdateCallback(func(projectID, agentType, containerName, sessionID string, cost float64, isEstimated bool) {
		called.containerName = containerName
		called.sessionID = sessionID
		called.cost = cost
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
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

func TestStore_HandleCostUpdate_CallbackWithZeroCost(t *testing.T) {
	var called bool
	var receivedCost float64

	store := NewStore(nil, nil)
	store.SetCostUpdateCallback(func(projectID, agentType, containerName, sessionID string, cost float64, isEstimated bool) {
		called = true
		receivedCost = cost
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 0, MessageCount: 0}),
		Timestamp:     time.Now(),
	})

	if !called {
		t.Error("cost update callback must be called even with zero cost")
	}
	if receivedCost != 0 {
		t.Errorf("expected cost 0, got %f", receivedCost)
	}
}

func TestStore_HandleCostUpdate_CallbackWithNilData(t *testing.T) {
	var called bool

	store := NewStore(nil, nil)
	store.SetCostUpdateCallback(func(projectID, agentType, containerName, sessionID string, cost float64, isEstimated bool) {
		called = true
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Data:          nil,
		Timestamp:     time.Now(),
	})

	if !called {
		t.Error("cost update callback must be called even with nil data")
	}
}

// ---------------------------------------------------------------------------
// Project-level attention broadcast
// ---------------------------------------------------------------------------

func TestStore_AttentionEmitsProjectStateBroadcast(t *testing.T) {
	broker := NewBroker()
	defer broker.Shutdown()
	ch, unsub := broker.Subscribe()
	defer unsub()

	store := NewStore(broker, nil)

	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	// Expect two broadcasts: worktree_state + project_state.
	var gotWorktree, gotProject bool
	for range 2 {
		select {
		case event := <-ch:
			switch event.Event {
			case SSEWorktreeState:
				gotWorktree = true
			case SSEProjectState:
				gotProject = true
				var payload ProjectStatePayload
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal project_state: %v", err)
				}
				if !payload.NeedsInput {
					t.Error("expected project needsInput=true")
				}
				if payload.NotificationType != engine.NotificationPermissionPrompt {
					t.Errorf("expected permission_prompt, got %s", payload.NotificationType)
				}
				if payload.ProjectID != "project-1" {
					t.Errorf("expected project-1, got %s", payload.ProjectID)
				}
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
	if !gotWorktree {
		t.Error("expected worktree_state broadcast")
	}
	if !gotProject {
		t.Error("expected project_state broadcast")
	}
}

func TestStore_AttentionClearEmitsProjectStateBroadcast(t *testing.T) {
	broker := NewBroker()
	defer broker.Shutdown()

	store := NewStore(broker, nil)

	// Set attention first (without subscribing yet — we don't need those broadcasts).
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	// Now subscribe and clear attention.
	ch, unsub := broker.Subscribe()
	defer unsub()

	store.HandleEvent(ContainerEvent{
		Type:          EventAttentionClear,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Expect project_state with needsInput=false.
	var gotProjectState bool
	for range 2 {
		select {
		case event := <-ch:
			if event.Event == SSEProjectState {
				gotProjectState = true
				var payload ProjectStatePayload
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if payload.NeedsInput {
					t.Error("expected project needsInput=false after clear")
				}
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
	if !gotProjectState {
		t.Error("expected project_state broadcast on attention clear")
	}
}

func TestStore_ProjectAttentionAggregatesHighestPriority(t *testing.T) {
	broker := NewBroker()
	defer broker.Shutdown()

	store := NewStore(broker, nil)

	// First worktree: idle_prompt (low priority).
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "wt-1",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	// Subscribe before the second event.
	ch, unsub := broker.Subscribe()
	defer unsub()

	// Second worktree: permission_prompt (high priority).
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "wt-2",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationPermissionPrompt}),
		Timestamp:     time.Now(),
	})

	// Find the project_state broadcast.
	for range 2 {
		select {
		case event := <-ch:
			if event.Event == SSEProjectState {
				var payload ProjectStatePayload
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if !payload.NeedsInput {
					t.Error("expected project needsInput=true")
				}
				if payload.NotificationType != engine.NotificationPermissionPrompt {
					t.Errorf("expected highest-priority permission_prompt, got %s", payload.NotificationType)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
	t.Error("expected project_state broadcast with aggregated attention")
}

func TestStore_ProjectAttentionIncludesCost(t *testing.T) {
	broker := NewBroker()
	defer broker.Shutdown()

	store := NewStore(broker, nil)

	// Record a cost first via a stop event.
	store.HandleEvent(ContainerEvent{
		Type:          EventCostUpdate,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, CostData{TotalCost: 5.25, SessionID: "sess-1"}),
		Timestamp:     time.Now(),
	})

	ch, unsub := broker.Subscribe()
	defer unsub()

	// Trigger an attention event.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "project-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	// The project_state broadcast should include both attention and cost.
	for range 2 {
		select {
		case event := <-ch:
			if event.Event == SSEProjectState {
				var payload ProjectStatePayload
				if err := json.Unmarshal(event.Data, &payload); err != nil {
					t.Fatalf("failed to unmarshal: %v", err)
				}
				if !payload.NeedsInput {
					t.Error("expected needsInput=true")
				}
				if payload.TotalCost != 5.25 {
					t.Errorf("expected cost 5.25, got %f", payload.TotalCost)
				}
				return
			}
		case <-time.After(time.Second):
			t.Fatal("timed out waiting for broadcast")
		}
	}
	t.Error("expected project_state broadcast")
}

// ---------------------------------------------------------------------------
// AliveCallback
// ---------------------------------------------------------------------------

func TestStore_AliveCallback_HeartbeatForUnknownContainer(t *testing.T) {
	var mu sync.Mutex
	var calledProjectID, calledContainer string

	store := NewStore(nil, nil)
	store.SetAliveCallback(func(projectID, agentType, containerName string) {
		mu.Lock()
		defer mu.Unlock()
		calledProjectID = projectID
		calledContainer = containerName
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	mu.Lock()
	defer mu.Unlock()
	if calledProjectID != "pid-1" {
		t.Errorf("expected projectID pid-1, got %q", calledProjectID)
	}
	if calledContainer != "proj-1" {
		t.Errorf("expected containerName proj-1, got %q", calledContainer)
	}
}

func TestStore_AliveCallback_SessionStartForUnknownContainer(t *testing.T) {
	var called bool

	store := NewStore(nil, nil)
	store.SetAliveCallback(func(projectID, agentType, containerName string) {
		called = true
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventSessionStart,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	if !called {
		t.Error("expected AliveCallback to fire on session_start for unknown container")
	}
}

func TestStore_AliveCallback_SecondHeartbeatDoesNotFire(t *testing.T) {
	var callCount int

	store := NewStore(nil, nil)
	store.SetAliveCallback(func(projectID, agentType, containerName string) {
		callCount++
	})

	// First heartbeat — should fire.
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Second heartbeat — should NOT fire.
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	if callCount != 1 {
		t.Errorf("expected AliveCallback to fire once, got %d", callCount)
	}
}

func TestStore_AliveCallback_FiresAgainAfterStale(t *testing.T) {
	var callCount int

	store := NewStore(nil, nil)
	store.SetAliveCallback(func(projectID, agentType, containerName string) {
		callCount++
	})

	// First heartbeat — fires.
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	// Mark stale — removes from lastEvents.
	store.MarkContainerStale("proj-1")

	// Heartbeat again — should fire because container was removed from lastEvents.
	store.HandleEvent(ContainerEvent{
		Type:          EventHeartbeat,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	if callCount != 2 {
		t.Errorf("expected AliveCallback to fire twice (before and after stale), got %d", callCount)
	}
}

func TestStore_AliveCallback_NonLifecycleEventDoesNotFire(t *testing.T) {
	var called bool

	store := NewStore(nil, nil)
	store.SetAliveCallback(func(projectID, agentType, containerName string) {
		called = true
	})

	// An attention event for an unknown container should NOT trigger alive.
	store.HandleEvent(ContainerEvent{
		Type:          EventAttention,
		ContainerName: "proj-1",
		ProjectID:     "pid-1",
		WorktreeID:    "main",
		Data:          mustMarshal(t, AttentionData{NotificationType: engine.NotificationIdlePrompt}),
		Timestamp:     time.Now(),
	})

	if called {
		t.Error("AliveCallback should not fire for non-lifecycle events")
	}
}

// ---------------------------------------------------------------------------
// writeToAuditLog: SourceID hash computation for JSONL dedup
// ---------------------------------------------------------------------------

func TestStore_WriteToAuditLog_SourceLineProducesSourceID(t *testing.T) {
	dbStore, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New() error: %v", err)
	}
	defer dbStore.Close() //nolint:errcheck

	// Use detailed mode so all events are written.
	allEvents := map[string]bool{"tool_use": true}
	writer := db.NewAuditWriter(dbStore, db.AuditDetailed, allEvents)
	store := NewStore(nil, writer)

	store.HandleEvent(ContainerEvent{
		Type:          EventToolUse,
		ContainerName: "proj-1",
		ProjectID:     "aabbccddee01",
		WorktreeID:    "main",
		Data:          mustMarshal(t, ToolUseData{ToolName: "bash"}),
		Timestamp:     time.Now(),
		SourceLine:    []byte(`{"type":"tool_use","tool":"bash"}`),
		SourceIndex:   0,
	})

	result, err := dbStore.Query(db.QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(result))
	}

	// SourceID is not returned by Query (it's json:"-"), so verify dedup
	// by writing the same event again — it should be silently dropped.
	store.HandleEvent(ContainerEvent{
		Type:          EventToolUse,
		ContainerName: "proj-1",
		ProjectID:     "aabbccddee01",
		WorktreeID:    "main",
		Data:          mustMarshal(t, ToolUseData{ToolName: "bash"}),
		Timestamp:     time.Now(),
		SourceLine:    []byte(`{"type":"tool_use","tool":"bash"}`),
		SourceIndex:   0,
	})

	result, err = dbStore.Query(db.QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 entry after duplicate (dedup), got %d", len(result))
	}
}

func TestStore_WriteToAuditLog_NoSourceLineProducesEmptySourceID(t *testing.T) {
	dbStore, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New() error: %v", err)
	}
	defer dbStore.Close() //nolint:errcheck

	allEvents := map[string]bool{"tool_use": true}
	writer := db.NewAuditWriter(dbStore, db.AuditDetailed, allEvents)
	store := NewStore(nil, writer)

	// Two identical events without SourceLine — both should be inserted
	// because empty SourceID becomes NULL (no uniqueness constraint on NULLs).
	for range 2 {
		store.HandleEvent(ContainerEvent{
			Type:          EventToolUse,
			ContainerName: "proj-1",
			ProjectID:     "aabbccddee01",
			WorktreeID:    "main",
			Data:          mustMarshal(t, ToolUseData{ToolName: "bash"}),
			Timestamp:     time.Now(),
			// No SourceLine set — hook-sourced event.
		})
	}

	result, err := dbStore.Query(db.QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (no SourceID = no dedup), got %d", len(result))
	}
}

func TestStore_WriteToAuditLog_SameLineDifferentIndex_DifferentSourceIDs(t *testing.T) {
	dbStore, err := db.New(t.TempDir())
	if err != nil {
		t.Fatalf("db.New() error: %v", err)
	}
	defer dbStore.Close() //nolint:errcheck

	allEvents := map[string]bool{"tool_use": true}
	writer := db.NewAuditWriter(dbStore, db.AuditDetailed, allEvents)
	store := NewStore(nil, writer)

	sourceLine := []byte(`{"type":"tool_use","tool":"bash"}`)

	// Same SourceLine but different SourceIndex values produce different hashes.
	store.HandleEvent(ContainerEvent{
		Type:          EventToolUse,
		ContainerName: "proj-1",
		ProjectID:     "aabbccddee01",
		WorktreeID:    "main",
		Data:          mustMarshal(t, ToolUseData{ToolName: "bash"}),
		Timestamp:     time.Now(),
		SourceLine:    sourceLine,
		SourceIndex:   0,
	})

	store.HandleEvent(ContainerEvent{
		Type:          EventToolUse,
		ContainerName: "proj-1",
		ProjectID:     "aabbccddee01",
		WorktreeID:    "main",
		Data:          mustMarshal(t, ToolUseData{ToolName: "bash"}),
		Timestamp:     time.Now(),
		SourceLine:    sourceLine,
		SourceIndex:   1,
	})

	result, err := dbStore.Query(db.QueryFilters{ProjectID: "aabbccddee01"})
	if err != nil {
		t.Fatalf("Query() error: %v", err)
	}
	if len(result) != 2 {
		t.Fatalf("expected 2 entries (same line, different index = different hash), got %d", len(result))
	}
}

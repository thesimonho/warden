package eventbus

import (
	"testing"
	"time"

	"github.com/thesimonho/warden/event"
)

func TestCheckContainerLiveness_SkipsRecentContainers(t *testing.T) {
	store := NewStore(nil, nil)

	// Simulate a recent heartbeat.
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventHeartbeat,
		ContainerName: "proj-1",
		Timestamp:     time.Now(),
	})
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	checkContainerLiveness(store)

	// Should still be active — heartbeat is recent.
	ws := store.GetWorktreeState("proj-1", "main")
	if !ws.SessionActive {
		t.Error("expected session to remain active for recent container")
	}
}

func TestCheckContainerLiveness_MarksStaleContainers(t *testing.T) {
	store := NewStore(nil, nil)

	// Simulate an old heartbeat (beyond the 30s threshold).
	staleTime := time.Now().Add(-45 * time.Second)
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     staleTime,
	})

	checkContainerLiveness(store)

	ws := store.GetWorktreeState("proj-1", "main")
	if ws.SessionActive {
		t.Error("expected session to be cleared for stale container")
	}
}

func TestCheckContainerLiveness_PreservesHealthyContainers(t *testing.T) {
	store := NewStore(nil, nil)

	// proj-1 is stale.
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventSessionStart,
		ContainerName: "proj-1",
		WorktreeID:    "main",
		Timestamp:     time.Now().Add(-45 * time.Second),
	})

	// proj-2 is fresh.
	store.HandleEvent(event.ContainerEvent{
		Type:          event.EventSessionStart,
		ContainerName: "proj-2",
		WorktreeID:    "main",
		Timestamp:     time.Now(),
	})

	checkContainerLiveness(store)

	ws1 := store.GetWorktreeState("proj-1", "main")
	if ws1.SessionActive {
		t.Error("expected proj-1 session to be stale")
	}

	ws2 := store.GetWorktreeState("proj-2", "main")
	if !ws2.SessionActive {
		t.Error("expected proj-2 session to remain active")
	}
}

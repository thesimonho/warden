package service

import (
	"testing"
	"time"

	"github.com/thesimonho/warden/api"
)

func TestFocusTracker_ReportFocusAndGet(t *testing.T) {
	ft := newFocusTracker(nil)

	// Initially empty.
	state := ft.getFocusState()
	if state.ActiveViewers != 0 {
		t.Errorf("expected 0 viewers, got %d", state.ActiveViewers)
	}

	// Report focus.
	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main", "feat-x"},
	})

	state = ft.getFocusState()
	if state.ActiveViewers != 1 {
		t.Errorf("expected 1 viewer, got %d", state.ActiveViewers)
	}

	key := "proj-a:claude-code"
	wts, ok := state.FocusedWorktrees[key]
	if !ok {
		t.Fatalf("expected focused worktrees for %s", key)
	}
	if len(wts) != 2 {
		t.Errorf("expected 2 worktrees, got %d", len(wts))
	}
}

func TestFocusTracker_Unfocus(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	// Unfocus.
	ft.reportFocus(api.FocusRequest{
		ClientID: "client-1",
		Focused:  false,
	})

	state := ft.getFocusState()
	if state.ActiveViewers != 0 {
		t.Errorf("expected 0 viewers after unfocus, got %d", state.ActiveViewers)
	}
}

func TestFocusTracker_UnfocusNonexistent(t *testing.T) {
	ft := newFocusTracker(nil)

	// Unfocus a client that was never focused — should be a no-op.
	ft.reportFocus(api.FocusRequest{
		ClientID: "ghost",
		Focused:  false,
	})

	state := ft.getFocusState()
	if state.ActiveViewers != 0 {
		t.Errorf("expected 0 viewers, got %d", state.ActiveViewers)
	}
}

func TestFocusTracker_HeartbeatRefreshesTTL(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	// Same state sent again (heartbeat) — should just refresh TTL without error.
	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	state := ft.getFocusState()
	if state.ActiveViewers != 1 {
		t.Errorf("expected 1 viewer after heartbeat, got %d", state.ActiveViewers)
	}
}

func TestFocusTracker_MultipleClients(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-2",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main", "feat-y"},
	})

	state := ft.getFocusState()
	if state.ActiveViewers != 2 {
		t.Errorf("expected 2 viewers, got %d", state.ActiveViewers)
	}

	// Deduplicated worktrees: main, feat-y.
	wts := state.FocusedWorktrees["proj-a:claude-code"]
	if len(wts) != 2 {
		t.Errorf("expected 2 deduplicated worktrees, got %d: %v", len(wts), wts)
	}
}

func TestFocusTracker_SweepExpired(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	// Artificially age the entry.
	ft.mu.Lock()
	ft.entries["client-1"].lastSeen = time.Now().Add(-2 * focusEntryTTL)
	ft.mu.Unlock()

	ft.sweepExpired()

	state := ft.getFocusState()
	if state.ActiveViewers != 0 {
		t.Errorf("expected 0 viewers after sweep, got %d", state.ActiveViewers)
	}
}

func TestFocusTracker_SweepKeepsFresh(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	// Entry is fresh — sweep should not remove it.
	ft.sweepExpired()

	state := ft.getFocusState()
	if state.ActiveViewers != 1 {
		t.Errorf("expected 1 viewer after sweep of fresh entry, got %d", state.ActiveViewers)
	}
}

func TestFocusTracker_UpdateChangesWorktrees(t *testing.T) {
	ft := newFocusTracker(nil)

	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main"},
	})

	// Client opens another worktree.
	ft.reportFocus(api.FocusRequest{
		ClientID:    "client-1",
		Focused:     true,
		ProjectID:   "proj-a",
		AgentType:   "claude-code",
		WorktreeIDs: []string{"main", "feat-z"},
	})

	state := ft.getFocusState()
	wts := state.FocusedWorktrees["proj-a:claude-code"]
	if len(wts) != 2 {
		t.Errorf("expected 2 worktrees after update, got %d: %v", len(wts), wts)
	}
}

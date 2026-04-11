package service

import (
	"context"
	"encoding/json"
	"log/slog"
	"slices"
	"sync"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/event"
	"github.com/thesimonho/warden/eventbus"
)

const (
	// focusEntryTTL is how long a focus entry survives without a heartbeat
	// refresh. Set to 1.5× the client heartbeat interval (30s) so one
	// missed beat doesn't evict an active client.
	focusEntryTTL = 45 * time.Second
	// focusCleanupInterval is how often the cleanup goroutine sweeps expired entries.
	focusCleanupInterval = 15 * time.Second
)

// focusEntry represents a single client's focus state.
type focusEntry struct {
	clientID    string
	projectID   string
	agentType   string
	worktreeIDs []string
	lastSeen    time.Time
}

// focusTracker tracks which clients are focused on which projects/worktrees.
// Used by the system tray to suppress desktop notifications for projects the
// user is actively viewing.
type focusTracker struct {
	broker  *eventbus.Broker
	mu      sync.Mutex
	entries map[string]*focusEntry // keyed by clientID
}

// newFocusTracker creates a focus tracker that broadcasts state changes via the broker.
func newFocusTracker(broker *eventbus.Broker) *focusTracker {
	return &focusTracker{
		broker:  broker,
		entries: make(map[string]*focusEntry),
	}
}

// startCleanup blocks, sweeping expired focus entries on a timer.
// Stops when the context is cancelled. Caller must wrap with `go`.
func (ft *focusTracker) startCleanup(ctx context.Context) {
	ticker := time.NewTicker(focusCleanupInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ft.sweepExpired()
		}
	}
}

// sweepExpired removes entries older than focusEntryTTL and broadcasts if changed.
func (ft *focusTracker) sweepExpired() {
	ft.mu.Lock()
	now := time.Now()
	changed := false
	for id, entry := range ft.entries {
		if now.Sub(entry.lastSeen) > focusEntryTTL {
			delete(ft.entries, id)
			changed = true
		}
	}
	ft.mu.Unlock()

	if changed {
		ft.broadcastState()
	}
}

// reportFocus updates or removes a client's focus entry and broadcasts if changed.
func (ft *focusTracker) reportFocus(req api.FocusRequest) {
	ft.mu.Lock()
	prev := ft.entries[req.ClientID]

	if !req.Focused {
		if prev == nil {
			ft.mu.Unlock()
			return
		}
		delete(ft.entries, req.ClientID)
		ft.mu.Unlock()
		ft.broadcastState()
		return
	}

	// Focused — upsert entry.
	entry := &focusEntry{
		clientID:    req.ClientID,
		projectID:   req.ProjectID,
		agentType:   req.AgentType,
		worktreeIDs: req.WorktreeIDs,
		lastSeen:    time.Now(),
	}

	// Check if anything meaningful changed (skip broadcast for pure heartbeats).
	if prev != nil && prev.projectID == entry.projectID &&
		prev.agentType == entry.agentType &&
		slices.Equal(prev.worktreeIDs, entry.worktreeIDs) {
		// Same focus state — just refresh the TTL.
		prev.lastSeen = entry.lastSeen
		ft.mu.Unlock()
		return
	}

	ft.entries[req.ClientID] = entry
	ft.mu.Unlock()
	ft.broadcastState()
}

// getFocusState returns the aggregated focus state across all clients.
func (ft *focusTracker) getFocusState() api.FocusState {
	ft.mu.Lock()
	defer ft.mu.Unlock()
	return ft.aggregateLocked()
}

// aggregateLocked computes the aggregated focus state. Caller must hold ft.mu.
func (ft *focusTracker) aggregateLocked() api.FocusState {
	state := api.FocusState{
		ActiveViewers: len(ft.entries),
	}

	if len(ft.entries) == 0 {
		return state
	}

	// Aggregate worktree IDs per project key, deduplicating across clients.
	perProject := make(map[string]map[string]struct{})
	for _, entry := range ft.entries {
		if entry.projectID == "" {
			continue
		}
		key := entry.projectID + ":" + entry.agentType
		wtSet, ok := perProject[key]
		if !ok {
			wtSet = make(map[string]struct{})
			perProject[key] = wtSet
		}
		for _, wt := range entry.worktreeIDs {
			wtSet[wt] = struct{}{}
		}
	}

	if len(perProject) > 0 {
		state.FocusedWorktrees = make(map[string][]string, len(perProject))
		for key, wtSet := range perProject {
			wts := make([]string, 0, len(wtSet))
			for wt := range wtSet {
				wts = append(wts, wt)
			}
			slices.Sort(wts)
			state.FocusedWorktrees[key] = wts
		}
	}

	return state
}

// --- Service-level methods ---

// ReportFocus updates or removes a client's focus entry.
func (s *Service) ReportFocus(req api.FocusRequest) {
	s.focus.reportFocus(req)
}

// GetFocusState returns the aggregated viewer focus state.
func (s *Service) GetFocusState() api.FocusState {
	return s.focus.getFocusState()
}

// StartFocusCleanup blocks, sweeping expired focus entries on a timer.
// Stops when the context is cancelled. Caller must wrap with `go`.
func (s *Service) StartFocusCleanup(ctx context.Context) {
	s.focus.startCleanup(ctx)
}

// broadcastState sends the current focus state to all SSE clients.
func (ft *focusTracker) broadcastState() {
	state := ft.getFocusState()

	if ft.broker == nil {
		return
	}

	data, err := json.Marshal(state)
	if err != nil {
		slog.Error("failed to marshal focus state", "error", err)
		return
	}

	ft.broker.Broadcast(event.SSEEvent{
		Event: event.SSEViewerFocus,
		Data:  data,
	})
}

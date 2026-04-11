package eventbus

import (
	"encoding/json"
	"log/slog"

	"github.com/thesimonho/warden/event"
)

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
		s.broker.Broadcast(event.SSEEvent{Event: b.event, Data: data})
	}
}

// broadcastBudgetEvent sends a budget enforcement SSE event with the shared
// [event.BudgetEventPayload] to all connected frontends.
func (s *Store) broadcastBudgetEvent(sseType event.SSEEventType, ref event.ProjectRef, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: sseType,
		data: event.BudgetEventPayload{
			ProjectRef: ref,
			TotalCost:  totalCost,
			Budget:     budget,
		},
	}})
}

// BroadcastWorktreeListChanged sends a worktree_list_changed event to all
// SSE clients so they can refresh the worktree list for the given container.
func (s *Store) BroadcastWorktreeListChanged(ref event.ProjectRef) {
	s.broadcast([]pendingBroadcast{{
		event: event.SSEWorktreeListChanged,
		data:  ref,
	}})
}

// BroadcastBudgetExceeded sends a budget_exceeded SSE event to all
// connected frontends so they can show a notification.
func (s *Store) BroadcastBudgetExceeded(ref event.ProjectRef, totalCost, budget float64) {
	s.broadcastBudgetEvent(event.SSEBudgetExceeded, ref, totalCost, budget)
}

// BroadcastBudgetContainerStopped sends a budget_container_stopped SSE event
// after a container is stopped due to budget enforcement, so frontends can
// redirect users away from the now-stopped project.
func (s *Store) BroadcastBudgetContainerStopped(ref event.ProjectRef, containerID string, totalCost, budget float64) {
	s.broadcast([]pendingBroadcast{{
		event: event.SSEBudgetContainerStopped,
		data: event.BudgetContainerStoppedPayload{
			BudgetEventPayload: event.BudgetEventPayload{
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
func buildWorktreeBroadcast(ref event.ProjectRef, worktreeID string, att *WorktreeState, ts *TerminalState) pendingBroadcast {
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

	return pendingBroadcast{event: event.SSEWorktreeState, data: payload}
}

// ProjectStatePayload is the JSON shape sent over SSE for project_state events.
// Carries both cost and attention state so the home page can update in real time.
type ProjectStatePayload struct {
	event.ProjectRef
	TotalCost            float64                `json:"totalCost"`
	MessageCount         int                    `json:"messageCount"`
	NeedsInput           bool                   `json:"needsInput"`
	NotificationType     event.NotificationType `json:"notificationType,omitempty"`
	AttentionWorktreeIDs []string               `json:"attentionWorktreeIDs,omitempty"`
}

// buildProjectBroadcast creates a project_state broadcast with complete state:
// aggregated attention across all worktrees plus current cost. Every project_state
// event carries the full snapshot so the frontend can apply it unconditionally.
// Must be called under lock.
func (s *Store) buildProjectBroadcast(ref event.ProjectRef) pendingBroadcast {
	needsInput, highestType, worktreeIDs := s.aggregateContainerAttention(ref.ContainerName)

	payload := ProjectStatePayload{
		ProjectRef:           ref,
		NeedsInput:           needsInput,
		NotificationType:     highestType,
		AttentionWorktreeIDs: worktreeIDs,
	}

	if cost, ok := s.costs[ref.ContainerName]; ok {
		payload.TotalCost = cost.TotalCost
		payload.MessageCount = cost.MessageCount
	}

	return pendingBroadcast{event: event.SSEProjectState, data: payload}
}

// BroadcastContainerStateChanged sends a container_state_changed SSE event
// so the tray and other clients can track container lifecycle without polling.
func (s *Store) BroadcastContainerStateChanged(ref event.ProjectRef, action event.ContainerStateAction) {
	s.broadcast([]pendingBroadcast{{
		event: event.SSEContainerStateChanged,
		data: event.ContainerStatePayload{
			ProjectRef: ref,
			Action:     action,
		},
	}})
}

// AggregateContainerAttention returns the highest-priority attention state
// across all worktrees for a container, along with the IDs of worktrees
// needing attention. The internal variant (lowercase) is used under
// existing lock; this public variant acquires its own read lock.
func (s *Store) AggregateContainerAttention(containerName string) (needsInput bool, highest event.NotificationType, worktreeIDs []string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.aggregateContainerAttention(containerName)
}

// aggregateContainerAttention returns the highest-priority attention state
// across all worktrees for a container, along with the IDs of worktrees
// needing attention. Must be called under lock.
func (s *Store) aggregateContainerAttention(containerName string) (needsInput bool, highest event.NotificationType, worktreeIDs []string) {
	for key, att := range s.attention {
		if key.containerName != containerName || !att.NeedsInput {
			continue
		}
		needsInput = true
		worktreeIDs = append(worktreeIDs, key.worktreeID)
		if highest == "" || event.NotificationPriority(att.NotificationType) > event.NotificationPriority(highest) {
			highest = att.NotificationType
		}
	}
	return
}

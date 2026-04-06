package eventbus

import (
	"encoding/json"
	"log/slog"
	"time"

	"github.com/thesimonho/warden/engine"
)

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
//
// Cost-triggered project_state broadcasts are throttled: the broadcast is
// suppressed if the cost delta since the last broadcast is < $0.01 AND
// less than 5s have elapsed. This reduces SSE noise during active agent
// work without delaying meaningful cost changes.
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

			// Throttle: only broadcast if cost changed meaningfully or enough time has passed.
			lastCost := s.lastBroadcastCost[event.ContainerName]
			lastTime := s.lastBroadcastTime[event.ContainerName]
			delta := parsed.TotalCost - lastCost
			if delta < 0 {
				delta = -delta
			}
			if delta >= costBroadcastMinDelta || time.Since(lastTime) >= costBroadcastMinInterval {
				broadcasts = append(broadcasts, s.buildProjectBroadcast(event.Ref()))
				s.lastBroadcastCost[event.ContainerName] = parsed.TotalCost
				s.lastBroadcastTime[event.ContainerName] = time.Now()
			}
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

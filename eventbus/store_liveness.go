package eventbus

import "time"

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

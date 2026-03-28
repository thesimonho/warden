package eventbus

import (
	"context"
	"log/slog"
	"time"
)

// Default liveness checker parameters.
const (
	// livenessCheckInterval is how often the checker runs.
	livenessCheckInterval = 15 * time.Second
	// livenessStalenessThreshold is the maximum time since the last event
	// before a container is considered stale (3 missed heartbeats).
	livenessStalenessThreshold = 30 * time.Second
)

// StartLivenessChecker runs a goroutine that periodically checks whether
// containers are still sending heartbeats. When a container misses enough
// heartbeats (exceeds the staleness threshold), all its worktree states
// are cleared and frontends are notified.
//
// Stops when ctx is cancelled.
func StartLivenessChecker(ctx context.Context, store *Store) {
	ticker := time.NewTicker(livenessCheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			checkContainerLiveness(store)
		}
	}
}

// checkContainerLiveness inspects all known containers and marks stale
// ones whose last event exceeds the threshold.
func checkContainerLiveness(store *Store) {
	containers := store.ActiveContainers()
	now := time.Now()

	for _, name := range containers {
		lastEvent := store.LastEventTime(name)
		if lastEvent.IsZero() {
			continue
		}

		if now.Sub(lastEvent) <= livenessStalenessThreshold {
			continue
		}

		slog.Warn("container heartbeat stale, marking worktrees disconnected",
			"container", name,
			"lastEvent", lastEvent,
			"staleness", now.Sub(lastEvent).Round(time.Second),
		)

		store.MarkContainerStale(name)
	}
}

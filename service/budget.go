package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/thesimonho/warden/db"
)

// ErrBudgetExceeded is returned when a project operation is blocked
// because the project has exceeded its cost budget.
var ErrBudgetExceeded = errors.New("project cost budget exceeded")

// GetEffectiveBudget returns the effective cost budget for a project.
// Uses per-project budget if > 0, otherwise the global default.
// Returns 0 (unlimited) if neither is set.
func (s *Service) GetEffectiveBudget(projectKey string) float64 {
	if s.db == nil {
		return 0
	}

	row, err := s.db.GetProject(projectKey)
	if err != nil || row == nil {
		return s.GetDefaultProjectBudget()
	}

	if row.CostBudget > 0 {
		return row.CostBudget
	}
	return s.GetDefaultProjectBudget()
}

// PersistSessionCost is the single gateway for all cost mutations.
// It persists session cost to the DB (when valid data is provided)
// and always triggers budget enforcement afterward.
//
// All code paths that write cost data MUST go through this method
// to guarantee enforcement is never skipped. This is analogous to
// how all audit writes go through [db.AuditWriter.Write].
//
// It is safe to call with empty sessionID or zero cost — the DB
// write is skipped but enforcement still runs against previously
// persisted data.
func (s *Service) PersistSessionCost(projectID, containerName, sessionID string, cost float64, isEstimated bool) {
	// Use projectID for DB operations when available, fall back to containerName.
	dbKey := projectID
	if dbKey == "" {
		slog.Warn("PersistSessionCost called without projectID, falling back to containerName",
			"containerName", containerName, "sessionID", sessionID)
		dbKey = containerName
	}
	if s.db != nil && sessionID != "" && cost > 0 {
		if err := s.db.UpsertSessionCost(dbKey, sessionID, cost, isEstimated); err != nil {
			slog.Error("failed to persist session cost", "projectID", dbKey, "session", sessionID, "err", err)
		}
	}
	s.enforceBudget(dbKey)
}

// enforceBudget checks whether a project has exceeded its cost budget
// and takes the configured enforcement actions. Called exclusively by
// [PersistSessionCost] to ensure all cost writes trigger enforcement.
func (s *Service) enforceBudget(projectKey string) {
	budget := s.GetEffectiveBudget(projectKey)
	if budget <= 0 {
		return // No budget set — unlimited.
	}

	// DB is the source of truth for cumulative cost.
	var effectiveCost float64
	if s.db != nil {
		if row, err := s.db.GetProjectTotalCost(projectKey); err == nil {
			effectiveCost = row.TotalCost
		}
	}

	// Look up the project row for container name (used in SSE payloads).
	var containerName string
	if s.db != nil {
		if row, err := s.db.GetProject(projectKey); err == nil && row != nil {
			containerName = effectiveContainerName(row)
		}
	}

	if effectiveCost <= budget {
		return // Within budget.
	}

	actions := s.getBudgetActions()

	if actions.warn {
		s.audit.Write(db.Entry{
			Source:    db.SourceBackend,
			Level:     db.LevelInfo,
			ProjectID: projectKey,
			Event:     "budget_exceeded",
			Message:   fmt.Sprintf("cost $%.2f exceeds budget $%.2f", effectiveCost, budget),
		})
		if s.store != nil {
			s.store.BroadcastBudgetExceeded(projectKey, containerName, effectiveCost, budget)
		}
	}

	if !actions.stopWorktrees && !actions.stopContainer {
		return // No destructive actions configured.
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	containerID := s.resolveContainerID(projectKey)
	if containerID == "" {
		s.audit.Write(db.Entry{
			Source:    db.SourceBackend,
			Level:     db.LevelError,
			ProjectID: projectKey,
			Event:     "budget_enforcement_failed",
			Message:   "could not resolve container ID for enforcement",
		})
		return
	}

	if actions.stopWorktrees {
		worktrees, err := s.docker.ListWorktrees(ctx, containerID, true)
		if err != nil {
			s.audit.Write(db.Entry{
				Source:    db.SourceBackend,
				Level:     db.LevelError,
				ProjectID: projectKey,
				Event:     "budget_enforcement_failed",
				Message:   fmt.Sprintf("listing worktrees failed: %v", err),
			})
		} else {
			for _, wt := range worktrees {
				if err := s.docker.KillWorktreeProcess(ctx, containerID, wt.ID); err != nil {
					s.audit.Write(db.Entry{
						Source:    db.SourceBackend,
						Level:     db.LevelError,
						ProjectID: projectKey,
						Event:     "budget_enforcement_failed",
						Message:   fmt.Sprintf("kill worktree %s failed: %v", wt.ID, err),
					})
				}
			}
			s.audit.Write(db.Entry{
				Source:    db.SourceBackend,
				Level:     db.LevelInfo,
				ProjectID: projectKey,
				Event:     "budget_worktrees_stopped",
				Message:   fmt.Sprintf("stopped %d worktrees (cost $%.2f exceeds budget $%.2f)", len(worktrees), effectiveCost, budget),
			})
		}
	}

	if actions.stopContainer {
		if err := s.docker.StopProject(ctx, containerID); err != nil {
			s.audit.Write(db.Entry{
				Source:    db.SourceBackend,
				Level:     db.LevelError,
				ProjectID: projectKey,
				Event:     "budget_enforcement_failed",
				Message:   fmt.Sprintf("stop container failed: %v", err),
			})
		} else {
			s.audit.Write(db.Entry{
				Source:    db.SourceBackend,
				Level:     db.LevelInfo,
				ProjectID: projectKey,
				Event:     "budget_container_stopped",
				Message:   fmt.Sprintf("container stopped (cost $%.2f exceeds budget $%.2f)", effectiveCost, budget),
			})
			if s.store != nil {
				s.store.BroadcastBudgetContainerStopped(projectKey, containerName, containerID, effectiveCost, budget)
			}
		}
	}
}

// IsOverBudget returns true if the project has exceeded its cost budget
// and the preventStart enforcement action is enabled.
func (s *Service) IsOverBudget(projectKey string) bool {
	if s.db == nil {
		return false
	}

	actions := s.getBudgetActions()
	if !actions.preventStart {
		return false
	}

	budget := s.GetEffectiveBudget(projectKey)
	if budget <= 0 {
		return false
	}

	row, err := s.db.GetProjectTotalCost(projectKey)
	if err != nil {
		return false
	}
	return row.TotalCost > budget
}

// resolveContainerID looks up the Docker container ID for a project from the DB.
// Returns empty string if the project has no container or is not found.
func (s *Service) resolveContainerID(projectKey string) string {
	if s.db == nil {
		return ""
	}
	row, err := s.db.GetProject(projectKey)
	if err != nil || row == nil {
		return ""
	}
	return row.ContainerID
}

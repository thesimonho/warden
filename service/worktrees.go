package service

import (
	"context"
	"time"

	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/eventbus"
)

// ListWorktrees returns all worktrees for the given project with
// their terminal state, enriched with real-time data from the event
// store when available.
func (s *Service) ListWorktrees(ctx context.Context, projectID, agentType string) ([]engine.Worktree, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	// Skip the expensive batch docker exec when the event bus store
	// already has terminal data for this container.
	skipEnrich := containerName != "" && s.store != nil && s.store.HasTerminalData(containerName)

	worktrees, err := s.docker.ListWorktrees(ctx, project.ContainerID, skipEnrich)
	if err != nil {
		return nil, err
	}

	if containerName != "" && len(worktrees) > 0 {
		s.overlayStoreState(containerName, worktrees)
	}

	return worktrees, nil
}

// CreateWorktree creates a new git worktree and connects a terminal.
func (s *Service) CreateWorktree(ctx context.Context, projectID, agentType, name string) (*WorktreeResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	worktreeID, err := s.docker.CreateWorktree(ctx, project.ContainerID, name, project.SkipPermissions)
	if err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      name,
			Event:         "worktree_create_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     project.ProjectID,
		AgentType:     project.AgentType,
		ContainerName: containerName,
		Worktree:      worktreeID,
		Event:         "worktree_created",
	})

	if containerName != "" && s.store != nil {
		s.store.BroadcastWorktreeListChanged(eventbus.ProjectRef{
			ProjectID: project.ProjectID, AgentType: project.AgentType, ContainerName: containerName,
		})
	}

	return &WorktreeResult{WorktreeID: worktreeID, ProjectID: project.ProjectID}, nil
}

// ConnectTerminal starts a terminal for a worktree in the given
// container. For background reconnects (abduco alive, no script
// needed), pushes a synthetic terminal_connected event so the store
// transitions from background to connected.
func (s *Service) ConnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*WorktreeResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	resultID, err := s.docker.ConnectTerminal(ctx, project.ContainerID, worktreeID, project.SkipPermissions)
	if err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      worktreeID,
			Event:         "terminal_connect_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	if containerName != "" && s.store != nil {
		s.store.HandleEvent(eventbus.ContainerEvent{
			Type:          eventbus.EventTerminalConnected,
			ContainerName: containerName,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			WorktreeID:    worktreeID,
			Timestamp:     time.Now(),
		})
	}

	return &WorktreeResult{WorktreeID: resultID, ProjectID: project.ProjectID}, nil
}

// DisconnectTerminal kills the terminal viewer for a worktree. The
// abduco session continues running in the background. Pushes a
// synthetic terminal_disconnected event so the store transitions
// from connected to background without relying on the container
// script's async curl delivery.
func (s *Service) DisconnectTerminal(ctx context.Context, projectID, agentType, worktreeID string) (*WorktreeResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	if err := s.docker.DisconnectTerminal(ctx, project.ContainerID, worktreeID); err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      worktreeID,
			Event:         "terminal_disconnect_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	if containerName != "" && s.store != nil {
		s.store.HandleEvent(eventbus.ContainerEvent{
			Type:          eventbus.EventTerminalDisconnected,
			ContainerName: containerName,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			WorktreeID:    worktreeID,
			Timestamp:     time.Now(),
		})
	}

	return &WorktreeResult{WorktreeID: worktreeID, ProjectID: project.ProjectID}, nil
}

// KillWorktreeProcess kills abduco and all child processes for a
// worktree, destroying the terminal entirely.
func (s *Service) KillWorktreeProcess(ctx context.Context, projectID, agentType, worktreeID string) (*WorktreeResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	if err := s.docker.KillWorktreeProcess(ctx, project.ContainerID, worktreeID); err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      worktreeID,
			Event:         "worktree_kill_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	if containerName != "" && s.store != nil {
		s.store.BroadcastWorktreeListChanged(eventbus.ProjectRef{
			ProjectID: project.ProjectID, AgentType: project.AgentType, ContainerName: containerName,
		})
	}

	return &WorktreeResult{WorktreeID: worktreeID, ProjectID: project.ProjectID}, nil
}

// RemoveWorktree fully removes a worktree: kills processes, runs
// `git worktree remove`, and cleans up tracking state.
func (s *Service) RemoveWorktree(ctx context.Context, projectID, agentType, worktreeID string) (*WorktreeResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	if err := s.docker.RemoveWorktree(ctx, project.ContainerID, worktreeID); err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      worktreeID,
			Event:         "worktree_remove_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	if containerName != "" && s.store != nil {
		s.store.EvictWorktree(containerName, worktreeID)
	}

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     project.ProjectID,
		AgentType:     project.AgentType,
		ContainerName: containerName,
		Worktree:      worktreeID,
		Event:         "worktree_removed",
	})

	if containerName != "" && s.store != nil {
		s.store.BroadcastWorktreeListChanged(eventbus.ProjectRef{
			ProjectID: project.ProjectID, AgentType: project.AgentType, ContainerName: containerName,
		})
	}

	return &WorktreeResult{WorktreeID: worktreeID, ProjectID: project.ProjectID}, nil
}

// CleanupWorktrees removes orphaned worktree directories and stale
// terminal tracking directories. Returns the list of removed IDs.
func (s *Service) CleanupWorktrees(ctx context.Context, projectID, agentType string) ([]string, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	removed, err := s.docker.CleanupOrphanedWorktrees(ctx, project.ContainerID)
	if err != nil {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelError,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Event:         "worktree_cleanup_failed",
			Message:       err.Error(),
		})
		return nil, err
	}

	if containerName != "" && s.store != nil {
		for _, wid := range removed {
			s.store.EvictWorktree(containerName, wid)
		}
	}

	for _, wid := range removed {
		s.audit.Write(db.Entry{
			Source:        db.SourceBackend,
			Level:         db.LevelInfo,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			ContainerName: containerName,
			Worktree:      wid,
			Event:         "worktree_cleaned_up",
		})
	}

	if containerName != "" && s.store != nil && len(removed) > 0 {
		s.store.BroadcastWorktreeListChanged(eventbus.ProjectRef{
			ProjectID: project.ProjectID, AgentType: project.AgentType, ContainerName: containerName,
		})
	}

	return removed, nil
}

// GetWorktreeDiff returns uncommitted changes for a worktree.
func (s *Service) GetWorktreeDiff(ctx context.Context, projectID, agentType, worktreeID string) (*api.DiffResponse, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	return s.docker.GetWorktreeDiff(ctx, project.ContainerID, worktreeID)
}

// NotifyTerminalDisconnected pushes a terminal_disconnected event to
// the event store. Called by the WebSocket handler when the last
// viewer closes.
func (s *Service) NotifyTerminalDisconnected(_ context.Context, project *db.ProjectRow, worktreeID string) {
	containerName := effectiveContainerName(project)
	if containerName != "" && s.store != nil {
		s.store.HandleEvent(eventbus.ContainerEvent{
			Type:          eventbus.EventTerminalDisconnected,
			ContainerName: containerName,
			ProjectID:     project.ProjectID,
			AgentType:     project.AgentType,
			WorktreeID:    worktreeID,
			Timestamp:     time.Now(),
		})
	}
}

// overlayStoreState merges event-bus state into the worktree list.
// Events arrive in real time and take precedence over file-based
// reads from the engine.
func (s *Service) overlayStoreState(containerName string, worktrees []engine.Worktree) {
	if s.store == nil {
		return
	}
	for i := range worktrees {
		ws := s.store.GetWorktreeState(containerName, worktrees[i].ID)

		if !ws.UpdatedAt.IsZero() && worktrees[i].State != engine.WorktreeStateDisconnected {
			worktrees[i].NeedsInput = ws.NeedsInput
			if ws.NeedsInput {
				worktrees[i].NotificationType = ws.NotificationType
			} else {
				worktrees[i].NotificationType = ""
			}

			if !ws.SessionActive && worktrees[i].State == engine.WorktreeStateConnected {
				worktrees[i].State = engine.WorktreeStateShell
			}
		}

		ts := s.store.GetTerminalState(containerName, worktrees[i].ID)
		derivedState := ts.DeriveWorktreeState()
		if derivedState == "" {
			continue
		}

		worktrees[i].State = derivedState
		if ts.ExitCode >= 0 {
			code := ts.ExitCode
			worktrees[i].ExitCode = &code
		}
	}
}

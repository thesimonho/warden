package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

// watcherCooldown prevents rapid watcher start/stop cycles during
// container crash-loops.
const watcherCooldown = 10 * time.Second

// startProjectWatcher resolves the agent type and workspace dir for a
// container and delegates to StartSessionWatcher. Reduces the repeated
// three-line normalization block at every lifecycle callsite.
func (s *Service) startProjectWatcher(projectID, containerName, agentType string) {
	if agentType == "" {
		agentType = string(agent.DefaultType)
	}
	workspaceDir := engine.ContainerWorkspaceDir(containerName)
	s.StartSessionWatcher(projectID, containerName, agentType, workspaceDir)
}

// StartSessionWatcher creates and starts a JSONL session file watcher
// for a project. The watcher tails session files, parses events, and
// feeds them into the eventbus pipeline via the event handler callback.
//
// No-op if the project is already being watched, or if the agent
// registry or event handler are not configured.
func (s *Service) StartSessionWatcher(projectID, containerName, agentType, workspaceDir string) {
	if s.agentRegistry == nil || s.eventHandler == nil {
		return
	}

	key := db.ProjectAgentKey{ProjectID: projectID, AgentType: agentType}

	s.sessionWatchersMu.Lock()
	defer s.sessionWatchersMu.Unlock()

	if _, exists := s.sessionWatchers[key]; exists {
		return
	}

	provider, ok := s.agentRegistry.Get(constants.AgentType(agentType))
	if !ok {
		return
	}

	parser := provider.NewSessionParser()
	if parser == nil {
		return
	}

	projectInfo := agent.ProjectInfo{
		ProjectID:    projectID,
		WorkspaceDir: workspaceDir,
		ProjectName:  containerName,
	}

	// Convert parsed JSONL events to container events and feed them
	// into the event pipeline. The worktree ID comes from the parsed
	// event when available, falling back to "main".
	eventHandler := s.eventHandler
	callback := func(event agent.ParsedEvent) {
		worktreeID := event.WorktreeID
		if worktreeID == "" {
			worktreeID = "main"
		}
		ctx := SessionContext{
			ProjectID:     projectID,
			ContainerName: containerName,
			AgentType:     agentType,
			WorktreeID:    worktreeID,
		}
		ce := SessionEventToContainerEvent(event, ctx)
		if ce != nil {
			eventHandler(*ce)
		}
	}

	sw := agent.NewSessionWatcher(parser, s.homeDir, projectInfo, callback)
	if err := sw.Start(context.Background()); err != nil {
		slog.Warn("failed to start session watcher", "project", projectID, "err", err)
		return
	}

	s.sessionWatchers[key] = sw
	delete(s.sessionWatcherCooldowns, key)
	slog.Info("started session watcher", "project", projectID, "agentType", agentType)
}

// StopSessionWatcher stops and removes the session watcher for a project+agent.
// Records a cooldown timestamp to prevent rapid restarts during crash-loops.
// No-op if no watcher is running for the given key.
func (s *Service) StopSessionWatcher(projectID, agentType string) {
	key := db.ProjectAgentKey{ProjectID: projectID, AgentType: agentType}

	s.sessionWatchersMu.Lock()
	defer s.sessionWatchersMu.Unlock()

	if sw, exists := s.sessionWatchers[key]; exists {
		sw.Stop()
		delete(s.sessionWatchers, key)
		s.sessionWatcherCooldowns[key] = time.Now()
		slog.Info("stopped session watcher", "project", projectID, "agentType", agentType)
	}
}

// RestartSessionWatcher stops any existing watcher for the project
// and starts a new one. Used when a container is restarted or renamed.
func (s *Service) RestartSessionWatcher(projectID, containerName, agentType, workspaceDir string) {
	s.StopSessionWatcher(projectID, agentType)
	s.StartSessionWatcher(projectID, containerName, agentType, workspaceDir)
}

// StopAllSessionWatchers stops all active session watchers. Called
// during graceful shutdown.
func (s *Service) StopAllSessionWatchers() {
	s.sessionWatchersMu.Lock()
	defer s.sessionWatchersMu.Unlock()

	for key, sw := range s.sessionWatchers {
		sw.Stop()
		delete(s.sessionWatchers, key)
	}
}

// HandleContainerAlive is called when the event bus detects a container
// sending events for the first time (or after being marked stale). It
// starts a session watcher if one isn't already running.
//
// This handles edge cases that ResumeSessionWatchers misses: containers
// that start after the server, containers that restart after being marked
// stale, and containers created by external tools.
func (s *Service) HandleContainerAlive(projectID, containerName string) {
	if s.agentRegistry == nil {
		return
	}

	// Quick in-memory check: if any watcher exists for this projectID,
	// skip the DB lookup entirely. This avoids a DB query on every heartbeat.
	s.sessionWatchersMu.Lock()
	alreadyWatching := false
	for key := range s.sessionWatchers {
		if key.ProjectID == projectID {
			alreadyWatching = true
			break
		}
	}
	s.sessionWatchersMu.Unlock()
	if alreadyWatching {
		return
	}

	// Look up the project to get the agent type.
	row, err := s.db.GetProjectByContainerName(containerName)
	if err != nil || row == nil {
		return
	}

	key := db.ProjectAgentKey{ProjectID: row.ProjectID, AgentType: row.AgentType}

	s.sessionWatchersMu.Lock()
	if lastStop, ok := s.sessionWatcherCooldowns[key]; ok {
		if time.Since(lastStop) < watcherCooldown {
			s.sessionWatchersMu.Unlock()
			return
		}
	}
	s.sessionWatchersMu.Unlock()

	slog.Info("container came alive, starting session watcher",
		"project", row.ProjectID, "container", containerName)
	s.startProjectWatcher(row.ProjectID, containerName, row.AgentType)
}

// ResumeSessionWatchers starts session watchers for all projects that
// have a running container. Called at startup so JSONL event parsing
// resumes without requiring a container restart.
func (s *Service) ResumeSessionWatchers(ctx context.Context) {
	if s.agentRegistry == nil {
		return
	}

	projects, err := s.ListProjects(ctx)
	if err != nil {
		slog.Warn("failed to list projects for session watcher resume", "err", err)
		return
	}

	for _, p := range projects {
		if p.State != "running" {
			continue
		}
		s.startProjectWatcher(p.ProjectID, p.Name, string(p.AgentType))
	}
}

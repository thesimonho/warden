package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/constants"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

// ListProjects returns all projects from the database, enriched with
// container state, DB metadata, and cost data from the event store.
func (s *Service) ListProjects(ctx context.Context) ([]engine.Project, error) {
	allRows, err := s.db.ListAllProjects()
	if err != nil {
		return nil, err
	}

	// Build container name list for Docker queries, and a name→row index.
	containerNames := make([]string, 0, len(allRows))
	rowsByName := make(map[string]*db.ProjectRow, len(allRows))
	for _, row := range allRows {
		name := effectiveContainerName(row)
		if name != "" {
			containerNames = append(containerNames, name)
			rowsByName[name] = row
		}
	}

	// Query Docker for container states.
	projects, err := s.docker.ListProjects(ctx, containerNames)
	if err != nil {
		return nil, err
	}

	// Overlay project identity and DB metadata.
	defaultBudget := s.GetDefaultProjectBudget()
	for i := range projects {
		if row, ok := rowsByName[projects[i].Name]; ok {
			projects[i].ProjectID = row.ProjectID
			projects[i].HostPath = row.HostPath
			applyDBMetadata(&projects[i], row, defaultBudget)
		}
	}

	// Also include projects with no container (tracked but container deleted/missing).
	projectNames := make(map[string]bool, len(projects))
	for _, p := range projects {
		projectNames[p.Name] = true
	}
	for _, row := range allRows {
		name := effectiveContainerName(row)
		if !projectNames[name] {
			p := engine.Project{
				ProjectID:    row.ProjectID,
				Name:         name,
				HostPath:     row.HostPath,
				HasContainer: false,
			}
			applyDBMetadata(&p, row, defaultBudget)
			projects = append(projects, p)
		}
	}

	s.overlayCost(ctx, projects)
	s.overlayAttention(projects)
	s.overlayAgentVersions(projects)
	return projects, nil
}

// AddProject registers a project in the database. The project ID is computed
// deterministically from the host path. If a project for this path and agent
// type already exists, returns the existing project without error.
func (s *Service) AddProject(name, hostPath, agentType string) (*ProjectResult, error) {
	projectID, err := engine.ProjectID(hostPath)
	if err != nil {
		return nil, fmt.Errorf("computing project ID: %w", err)
	}

	if agentType == "" {
		agentType = string(constants.DefaultAgentType)
	}

	has, err := s.db.HasProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	if has {
		return &ProjectResult{ProjectID: projectID, Name: name, AgentType: agentType}, nil
	}
	if err := s.db.InsertProject(db.ProjectRow{
		ProjectID: projectID,
		Name:      name,
		HostPath:  hostPath,
		AgentType: agentType,
	}); err != nil {
		return nil, err
	}
	return &ProjectResult{ProjectID: projectID, Name: name, AgentType: agentType}, nil
}

// RemoveProject removes a project from the database by compound key.
// When audit logging is enabled, cost data and events are preserved so the
// audit log remains accurate. When audit logging is off, all associated
// data is cleaned up.
func (s *Service) RemoveProject(projectID, agentType string) (*ProjectResult, error) {
	// Look up the project name before deleting for the result.
	var name string
	if row, err := s.db.GetProject(projectID, agentType); err == nil && row != nil {
		name = effectiveContainerName(row)
	}

	if err := s.db.DeleteProject(projectID, agentType); err != nil {
		return nil, err
	}

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: name,
		Event:         "project_removed",
		Message:       fmt.Sprintf("project %q removed from Warden", name),
	})

	if s.GetAuditLogMode() == api.AuditLogOff {
		// Audit logging is off — clean up all associated data.
		if err := s.db.DeleteProjectCosts(projectID, agentType); err != nil {
			slog.Warn("failed to delete project costs", "projectID", projectID, "err", err)
		}
		if _, err := s.DeleteAuditEvents(api.AuditFilters{ProjectID: projectID}); err != nil {
			slog.Warn("failed to delete project events", "projectID", projectID, "err", err)
		}
	}

	return &ProjectResult{ProjectID: projectID, Name: name, AgentType: agentType}, nil
}

// ResetProjectCosts removes all cost history for a project+agent pair.
// This is an audit event itself — the fact that costs were reset is recorded.
func (s *Service) ResetProjectCosts(projectID, agentType string) error {
	if s.db == nil {
		return nil
	}

	name := s.resolveProjectName(projectID, agentType)
	if err := s.db.DeleteProjectCosts(projectID, agentType); err != nil {
		return err
	}
	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: name,
		Event:         "cost_reset",
		Message:       "project cost history cleared",
	})
	return nil
}

// PurgeProjectAudit removes all audit events for a project.
// The audit_purged event is written before the purge but will be deleted
// by it — the event serves as a write-ahead record for external log
// consumers that process events before they are purged.
func (s *Service) PurgeProjectAudit(projectID, agentType string) (int64, error) {
	name := s.resolveProjectName(projectID, agentType)
	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: name,
		Event:         "audit_purged",
		Message:       "project audit history purged",
	})
	return s.DeleteAuditEvents(api.AuditFilters{ProjectID: projectID})
}

// GetProject returns a project row by compound key, or nil if not found.
func (s *Service) GetProject(projectID, agentType string) (*db.ProjectRow, error) {
	if s.db == nil {
		return nil, nil
	}
	return s.db.GetProject(projectID, agentType)
}

// StopProject stops the container for the given project. Before stopping,
// it captures cost from the agent's config file via docker exec and
// persists it to the DB so cost data survives the container stop.
func (s *Service) StopProject(
	ctx context.Context,
	projectID, agentType string,
) (*ProjectResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)
	costResult := s.readAndPersistAgentCost(
		ctx,
		project.ProjectID,
		project.AgentType,
		project.ContainerID,
		containerName,
	)
	s.writeCostSnapshot(project.ProjectID, project.AgentType, containerName, costResult)

	if err := s.docker.StopProject(ctx, project.ContainerID); err != nil {
		return nil, err
	}

	s.StopSessionWatcher(project.ProjectID, project.AgentType)

	return &ProjectResult{
		ContainerID: project.ContainerID,
		ProjectID:   project.ProjectID,
		AgentType:   project.AgentType,
		Name:        containerName,
	}, nil
}

// RestartProject restarts the container for the given project. If bind mount
// sources are stale (e.g. after a Nix Home Manager generation switch),
// the restart is blocked and a StaleMountsError is returned so the UI
// can warn the user. Returns ErrBudgetExceeded if the project is over
// budget and the preventStart enforcement action is enabled.
func (s *Service) RestartProject(
	ctx context.Context,
	projectID, agentType string,
) (*ProjectResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	if s.IsOverBudget(project.ProjectID, project.AgentType) {
		return nil, ErrBudgetExceeded
	}

	// Read original mounts from DB for stale mount validation.
	var originalMounts []api.Mount
	if len(project.OriginalMounts) > 0 {
		if unmarshalErr := json.Unmarshal(
			project.OriginalMounts,
			&originalMounts,
		); unmarshalErr != nil {
			slog.Warn(
				"failed to decode original mounts",
				"name",
				containerName,
				"err",
				unmarshalErr,
			)
		}
	}

	if err := s.docker.RestartProject(ctx, project.ContainerID, originalMounts); err != nil {
		var staleErr *engine.StaleMountsError
		if errors.As(err, &staleErr) {
			s.audit.Write(db.Entry{
				Source:        db.SourceBackend,
				Level:         db.LevelError,
				ProjectID:     project.ProjectID,
				AgentType:     project.AgentType,
				ContainerName: containerName,
				Event:         "restart_blocked_stale_mounts",
				Message:       "bind mounts are stale — recreate the container to refresh mounts",
				Attrs:         map[string]any{"stalePaths": staleErr.StalePaths},
			})
		}
		return nil, err
	}
	s.StopSessionWatcher(project.ProjectID, project.AgentType)
	s.startProjectWatcher(project.ProjectID, containerName, project.AgentType)

	return &ProjectResult{
		ProjectID:   project.ProjectID,
		AgentType:   project.AgentType,
		Name:        containerName,
		ContainerID: project.ContainerID,
	}, nil
}

// applyDBMetadata merges database-stored project metadata onto a single project.
// defaultBudget is the global fallback (pass 0 if not needed).
func applyDBMetadata(p *engine.Project, row *db.ProjectRow, defaultBudget float64) {
	p.AgentType = constants.AgentType(row.AgentType)
	p.SkipPermissions = row.SkipPermissions
	if row.NetworkMode != "" {
		p.NetworkMode = api.NetworkMode(row.NetworkMode)
	}
	if row.AllowedDomains != "" {
		p.AllowedDomains = splitCSV(row.AllowedDomains)
	}
	if row.HostPath != "" && p.MountedDir == "" {
		p.MountedDir = row.HostPath
	}
	if row.CostBudget > 0 {
		p.CostBudget = row.CostBudget
	} else if defaultBudget > 0 {
		p.CostBudget = defaultBudget
	}
}

// HandleContainerStale writes an audit entry when a container's heartbeat
// goes stale. Called by the event bus stale callback so the audit entry
// includes full project context (project ID and container name).
//
// When the container is crash-looping, an additional container_startup_failed
// event is written with the container's log tail for diagnostics.
func (s *Service) HandleContainerStale(containerName string) {
	var projectID, agentType string
	if s.db != nil {
		if row, err := s.db.GetProjectByContainerName(containerName); err == nil && row != nil {
			projectID = row.ProjectID
			agentType = row.AgentType
			containerName = effectiveContainerName(row)
		}
	}

	// The session watcher is NOT stopped here. It tails host-side JSONL
	// files which persist regardless of container state, and the 2s poll
	// is essentially free when no new data arrives. Stopping it on stale
	// creates a race: if the container is deleted and recreated quickly,
	// the liveness checker fires the stale callback for the OLD container
	// but looks up the NEW project by name, killing its watcher.
	// Watchers are stopped only by explicit lifecycle events (delete,
	// stop, shutdown).

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelWarn,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: containerName,
		Event:         "container_heartbeat_stale",
		Message:       "container heartbeat stale, marking worktrees disconnected",
	})

	// Check if the container is crash-looping and capture diagnostics.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	health, err := s.docker.ContainerStartupHealth(ctx, containerName)
	if err != nil {
		return
	}
	if !health.Restarting && health.RestartCount == 0 && !health.OOMKilled {
		return
	}

	msg := fmt.Sprintf(
		"container crash-looping (restarts: %d, exit code: %d, OOM: %v)",
		health.RestartCount, health.ExitCode, health.OOMKilled,
	)

	entry := db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelError,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: containerName,
		Event:         "container_startup_failed",
		Message:       msg,
	}
	if health.LogTail != "" {
		entry.Data = []byte(health.LogTail)
	}
	s.audit.Write(entry)
}

// effectiveContainerName returns the Docker container name for a project row.
// Prefers ContainerName (explicitly stored after creation), falls back to Name.
func effectiveContainerName(row *db.ProjectRow) string {
	if row.ContainerName != "" {
		return row.ContainerName
	}
	return row.Name
}

// resolveProjectName looks up the container name for a project by compound key.
// Returns empty string if the project is not found.
func (s *Service) resolveProjectName(projectID, agentType string) string {
	if s.db == nil {
		return ""
	}
	row, err := s.db.GetProject(projectID, agentType)
	if err != nil || row == nil {
		return ""
	}
	return effectiveContainerName(row)
}

// overlayAttention merges event-bus attention state onto the project list.
// Uses the same aggregation logic as the SSE broadcast path to keep
// poll-based and push-based results consistent.
// overlayAgentVersions sets the pinned CLI version on each project
// based on its agent type. The version matches what was passed to the
// container at creation time via WARDEN_CLAUDE_VERSION / WARDEN_CODEX_VERSION.
func (s *Service) overlayAgentVersions(projects []engine.Project) {
	for i := range projects {
		if !projects[i].HasContainer {
			continue
		}
		projects[i].AgentVersion = agent.VersionForType(projects[i].AgentType)
	}
}

func (s *Service) overlayAttention(projects []engine.Project) {
	if s.store == nil {
		return
	}
	for i := range projects {
		if projects[i].State != "running" {
			continue
		}
		projects[i].NeedsInput, projects[i].NotificationType = s.store.AggregateContainerAttention(
			projects[i].Name,
		)
	}
}

// splitCSV splits a comma-separated string into a slice.
// Returns nil for empty strings.
func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, ",")
}

// overlayCost merges cost data into the project list.
// Primary source: cumulative session costs in the DB.
// Fallback for running containers with no DB data: read the agent's
// config file via docker exec and persist to DB for future reads.
func (s *Service) overlayCost(ctx context.Context, projects []engine.Project) {
	if len(projects) == 0 {
		return
	}

	// Batch-load all DB costs in a single query (avoids N+1).
	var dbCosts map[db.ProjectAgentKey]db.ProjectCostRow
	if s.db != nil {
		var err error
		dbCosts, err = s.db.GetAllProjectTotalCosts()
		if err != nil {
			slog.Debug("failed to load project costs from DB", "err", err)
		}
	}

	for i := range projects {
		// Primary: cumulative cost from DB (session_costs table).
		key := db.ProjectAgentKey{
			ProjectID: projects[i].ProjectID,
			AgentType: string(projects[i].AgentType),
		}
		if row, ok := dbCosts[key]; ok && row.TotalCost > 0 {
			projects[i].TotalCost = row.TotalCost
			projects[i].IsEstimatedCost = row.IsEstimated
			continue
		}

		// Fallback: read from running container and persist to DB.
		if projects[i].State != "running" {
			continue
		}
		result := s.readAndPersistAgentCost(
			ctx,
			projects[i].ProjectID,
			string(projects[i].AgentType),
			projects[i].ID,
			projects[i].Name,
		)
		if result != nil && result.TotalCost > 0 {
			projects[i].TotalCost = result.TotalCost
			projects[i].IsEstimatedCost = result.IsEstimated
		}
	}
}

// readAndPersistAgentCost reads cost from the agent's config file via
// docker exec and persists per-session costs to the DB. Budget enforcement
// is triggered once after all sessions are persisted.
// Returns the result for the caller to use. Best-effort — errors are logged.
func (s *Service) readAndPersistAgentCost(
	ctx context.Context,
	projectID, agentType, containerID, containerName string,
) *engine.AgentCostResult {
	result, err := s.docker.ReadAgentCostAndBillingType(
		ctx,
		containerID,
		engine.ContainerWorkspaceDir(containerName),
	)
	if err != nil {
		slog.Debug("agent cost read failed", "container", containerName, "err", err)
		return nil
	}

	// Persist each session's cost keyed by projectID+agentType, then enforce budget once.
	if s.db != nil {
		for _, sc := range result.Sessions {
			if sc.SessionID != "" && sc.Cost > 0 {
				if err := s.db.UpsertSessionCost(
					projectID,
					agentType,
					sc.SessionID,
					sc.Cost,
					result.IsEstimated,
				); err != nil {
					slog.Debug(
						"failed to persist session cost",
						"projectID",
						projectID,
						"session",
						sc.SessionID,
						"err",
						err,
					)
				}
			}
		}
	}
	s.enforceBudget(projectID, agentType)

	return result
}

// writeCostSnapshot writes a cost_snapshot audit entry at container stop.
// Uses the docker exec result if available (Claude Code), otherwise falls
// back to the DB for agents that only report cost via JSONL (Codex).
func (s *Service) writeCostSnapshot(
	projectID, agentType, containerName string,
	costResult *engine.AgentCostResult,
) {
	totalCost := 0.0
	sessionCount := 0
	isEstimated := true

	if costResult != nil && costResult.TotalCost > 0 {
		totalCost = costResult.TotalCost
		sessionCount = len(costResult.Sessions)
		isEstimated = costResult.IsEstimated
	} else if s.db != nil {
		// Fallback: query DB for cost already persisted via JSONL token updates.
		costRow, err := s.db.GetProjectTotalCost(projectID, agentType)
		if err != nil {
			slog.Warn(
				"failed to query project cost for snapshot",
				"projectID",
				projectID,
				"err",
				err,
			)
			return
		}
		totalCost = costRow.TotalCost
		isEstimated = costRow.IsEstimated
	}

	if totalCost <= 0 {
		return
	}
	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     projectID,
		AgentType:     agentType,
		ContainerName: containerName,
		Event:         "cost_snapshot",
		Message: fmt.Sprintf(
			"cost at container stop: $%.2f (sessions: %d, estimated: %v)",
			totalCost,
			sessionCount,
			isEstimated,
		),
		Attrs: map[string]any{
			"totalCost":    totalCost,
			"sessionCount": sessionCount,
			"isEstimated":  isEstimated,
		},
	})
}

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
)

// CreateContainer creates a new project container and saves full
// project metadata to the database.
func (s *Service) CreateContainer(ctx context.Context, req engine.CreateContainerRequest) (*ContainerResult, error) {
	row, err := projectRowFromRequest(req)
	if err != nil {
		return nil, err
	}

	// Resolve enabled access items into env vars and mounts.
	if err := s.ResolveAccessItemsForContainer(&req); err != nil {
		return nil, fmt.Errorf("resolving access items: %w", err)
	}

	// OriginalMounts must include access item mounts so that stale mount
	// detection compares against the full set Docker actually receives.
	// row.Mounts stays user-only so InspectContainer doesn't duplicate
	// access item mounts when the user edits and re-saves.
	if len(req.Mounts) > 0 {
		if data, err := json.Marshal(req.Mounts); err == nil {
			row.OriginalMounts = data
		}
	}

	containerID, err := s.docker.CreateContainer(ctx, req)
	if err != nil {
		return nil, err
	}

	// Store the Docker container ID/name on the project row.
	row.ContainerID = containerID
	row.ContainerName = req.Name

	if insertErr := s.db.InsertProject(row); insertErr != nil {
		slog.Warn("container created but failed to save to db", "name", req.Name, "err", insertErr)
	}

	return &ContainerResult{ContainerID: containerID, Name: req.Name, ProjectID: row.ProjectID}, nil
}

// DeleteContainer stops and removes a container.
func (s *Service) DeleteContainer(ctx context.Context, project *db.ProjectRow) (*ContainerResult, error) {
	containerName := effectiveContainerName(project)

	if err := s.docker.DeleteContainer(ctx, project.ContainerID); err != nil {
		return nil, err
	}

	// Clean up the event directory for this container.
	s.docker.CleanupEventDir(containerName)

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     project.ProjectID,
		ContainerName: containerName,
		Event:         "container_deleted",
		Message:       fmt.Sprintf("container %q deleted", containerName),
	})

	return &ContainerResult{ContainerID: project.ContainerID, Name: containerName, ProjectID: project.ProjectID}, nil
}

// InspectContainer returns the editable configuration of a container.
// Docker-derived fields come from the engine; DB metadata is overlaid
// directly from the project row.
func (s *Service) InspectContainer(ctx context.Context, project *db.ProjectRow) (*engine.ContainerConfig, error) {
	cfg, err := s.docker.InspectContainer(ctx, project.ContainerID)
	if err != nil {
		return nil, err
	}

	// Overlay DB metadata. The DB stores user-provided env vars and
	// pre-symlink-resolution mounts. Using the DB values prevents
	// access-item-injected env vars and symlink-resolved mounts from
	// leaking into the editable config (which would duplicate them
	// when the user saves — the resolver re-expands on create).
	cfg.SkipPermissions = project.SkipPermissions
	if project.NetworkMode != "" {
		cfg.NetworkMode = engine.NetworkMode(project.NetworkMode)
	}
	if project.AllowedDomains != "" {
		cfg.AllowedDomains = splitCSV(project.AllowedDomains)
	}
	if len(project.EnvVars) > 0 {
		var envVars map[string]string
		if err := json.Unmarshal(project.EnvVars, &envVars); err == nil {
			cfg.EnvVars = envVars
		}
	} else {
		cfg.EnvVars = nil
	}
	if len(project.Mounts) > 0 {
		var mounts []engine.Mount
		if err := json.Unmarshal(project.Mounts, &mounts); err == nil {
			cfg.Mounts = mounts
		}
	} else {
		cfg.Mounts = nil
	}
	cfg.CostBudget = project.CostBudget
	if project.EnabledAccessItems != "" {
		cfg.EnabledAccessItems = splitCSV(project.EnabledAccessItems)
	}

	return cfg, nil
}

// UpdateContainer recreates a container with updated configuration
// and updates the database row.
func (s *Service) UpdateContainer(ctx context.Context, project *db.ProjectRow, req engine.CreateContainerRequest) (*ContainerResult, error) {
	row, err := projectRowFromRequest(req)
	if err != nil {
		return nil, err
	}

	// Resolve enabled access items into env vars and mounts.
	if err := s.ResolveAccessItemsForContainer(&req); err != nil {
		return nil, fmt.Errorf("resolving access items: %w", err)
	}

	// Update OriginalMounts to include access item mounts (see CreateContainer).
	if len(req.Mounts) > 0 {
		if data, err := json.Marshal(req.Mounts); err == nil {
			row.OriginalMounts = data
		}
	}

	newID, err := s.docker.RecreateContainer(ctx, project.ContainerID, req)
	if err != nil {
		return nil, err
	}

	// Update DB row with new config and container ID.
	row.ContainerID = newID
	row.ContainerName = req.Name
	if insertErr := s.db.InsertProject(row); insertErr != nil {
		slog.Warn("container recreated but failed to update db", "name", req.Name, "err", insertErr)
	}

	return &ContainerResult{ContainerID: newID, Name: req.Name, ProjectID: row.ProjectID}, nil
}

// ValidateContainer checks whether a container has the required Warden
// terminal infrastructure installed.
func (s *Service) ValidateContainer(ctx context.Context, project *db.ProjectRow) (*ValidateContainerResult, error) {
	valid, missing, err := s.docker.ValidateInfrastructure(ctx, project.ContainerID)
	if err != nil {
		return nil, err
	}
	return &ValidateContainerResult{Valid: valid, Missing: missing}, nil
}

// projectRowFromRequest converts a CreateContainerRequest to a ProjectRow
// for database persistence. Computes the deterministic ProjectID from the
// host path.
func projectRowFromRequest(req engine.CreateContainerRequest) (db.ProjectRow, error) {
	projectID, err := engine.ProjectID(req.ProjectPath)
	if err != nil {
		return db.ProjectRow{}, fmt.Errorf("computing project ID: %w", err)
	}

	agentType := req.AgentType
	if agentType == "" {
		agentType = agent.DefaultAgentType
	}

	row := db.ProjectRow{
		ProjectID:       projectID,
		Name:            req.Name,
		AddedAt:         time.Now().UTC(),
		Image:           req.Image,
		HostPath:        req.ProjectPath,
		AgentType:       agentType,
		SkipPermissions: req.SkipPermissions,
		NetworkMode:     string(req.NetworkMode),
	}

	if len(req.EnvVars) > 0 {
		if data, err := json.Marshal(req.EnvVars); err == nil {
			row.EnvVars = data
		}
	}
	if len(req.Mounts) > 0 {
		if data, err := json.Marshal(req.Mounts); err == nil {
			row.Mounts = data
			// OriginalMounts stores pre-symlink-resolution specs so
			// RestartProject can detect stale bind mounts after a
			// dotfile manager changes symlink targets.
			row.OriginalMounts = data
		}
	}
	if len(req.AllowedDomains) > 0 {
		row.AllowedDomains = strings.Join(req.AllowedDomains, ",")
	}
	row.CostBudget = req.CostBudget
	if len(req.EnabledAccessItems) > 0 {
		row.EnabledAccessItems = strings.Join(req.EnabledAccessItems, ",")
	}

	return row, nil
}

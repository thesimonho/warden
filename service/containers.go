package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

	// Overlay DB metadata. The DB stores pre-symlink-resolution mounts,
	// while Docker's bind list contains resolved extras. Using the DB
	// mounts prevents duplicate mount points when the user edits and
	// saves (the resolver would re-expand symlinks, doubling the extras).
	cfg.SkipPermissions = project.SkipPermissions
	if project.NetworkMode != "" {
		cfg.NetworkMode = engine.NetworkMode(project.NetworkMode)
	}
	if project.AllowedDomains != "" {
		cfg.AllowedDomains = splitDomains(project.AllowedDomains)
	}
	if len(project.Mounts) > 0 {
		var mounts []engine.Mount
		if err := json.Unmarshal(project.Mounts, &mounts); err == nil {
			cfg.Mounts = mounts
		}
	}
	cfg.CostBudget = project.CostBudget

	return cfg, nil
}

// UpdateContainer recreates a container with updated configuration
// and updates the database row.
func (s *Service) UpdateContainer(ctx context.Context, project *db.ProjectRow, req engine.CreateContainerRequest) (*ContainerResult, error) {
	row, err := projectRowFromRequest(req)
	if err != nil {
		return nil, err
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

	row := db.ProjectRow{
		ProjectID:       projectID,
		Name:            req.Name,
		AddedAt:         time.Now().UTC(),
		Image:           req.Image,
		HostPath:        req.ProjectPath,
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

	return row, nil
}

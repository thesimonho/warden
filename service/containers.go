package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/thesimonho/warden/agent"
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/db"
	"github.com/thesimonho/warden/engine"
	"github.com/thesimonho/warden/runtimes"
)

// CreateContainer creates a new project container and saves full
// project metadata to the database.
func (s *Service) CreateContainer(ctx context.Context, req api.CreateContainerRequest) (*ContainerResult, error) {
	row, err := projectRowFromRequest(req)
	if err != nil {
		return nil, err
	}

	// Validate runtime IDs before passing to the engine.
	for _, id := range req.EnabledRuntimes {
		if !runtimes.IsValidID(id) {
			return nil, fmt.Errorf("unknown runtime: %q", id)
		}
	}

	// Resolve enabled access items into env vars and mounts.
	if err := s.ResolveAccessItemsForContainer(&req); err != nil {
		return nil, fmt.Errorf("resolving access items: %w", err)
	}

	// Merge runtime domains into the allowed list for restricted mode.
	// This runs after projectRowFromRequest so the DB stores only user-entered
	// domains; merged runtime domains are only passed to the Docker engine.
	mergeRuntimeDomains(&req)

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

	// Start lifecycle watchers for the new container.
	if s.eventWatcher != nil {
		s.eventWatcher.WatchContainerDir(req.Name)
	}
	s.startProjectWatcher(row.ProjectID, req.Name, string(req.AgentType))

	configData, _ := json.Marshal(map[string]any{
		"image":              req.Image,
		"projectPath":        req.ProjectPath,
		"networkMode":        req.NetworkMode,
		"allowedDomains":     req.AllowedDomains,
		"enabledRuntimes":    req.EnabledRuntimes,
		"enabledAccessItems": req.EnabledAccessItems,
		"skipPermissions":    req.SkipPermissions,
		"costBudget":         req.CostBudget,
	})
	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     row.ProjectID,
		AgentType:     row.AgentType,
		ContainerName: req.Name,
		Event:         "container_created",
		Message:       fmt.Sprintf("container %q created", req.Name),
		Data:          configData,
	})

	return &ContainerResult{ContainerID: containerID, Name: req.Name, ProjectID: row.ProjectID}, nil
}

// DeleteContainer stops and removes a container.
func (s *Service) DeleteContainer(ctx context.Context, projectID, agentType string) (*ContainerResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}
	containerName := effectiveContainerName(project)

	if err := s.docker.DeleteContainer(ctx, project.ContainerID); err != nil {
		return nil, err
	}

	// Clean up the event directory for this container.
	s.docker.CleanupEventDir(containerName)

	// Stop lifecycle watchers for the deleted container.
	s.StopSessionWatcher(project.ProjectID, project.AgentType)
	if s.eventWatcher != nil {
		s.eventWatcher.CleanupContainerDir(containerName)
	}
	// Clear the container from the event store so the liveness checker
	// doesn't find a stale entry for this name and inadvertently stop
	// a newly created container's session watcher.
	if s.store != nil {
		s.store.RemoveContainer(containerName)
	}

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     project.ProjectID,
		AgentType:     project.AgentType,
		ContainerName: containerName,
		Event:         "container_deleted",
		Message:       fmt.Sprintf("container %q deleted", containerName),
	})

	return &ContainerResult{ContainerID: project.ContainerID, Name: containerName, ProjectID: project.ProjectID}, nil
}

// InspectContainer returns the editable configuration of a container.
// Docker-derived fields come from the engine; DB metadata is overlaid
// directly from the project row.
func (s *Service) InspectContainer(ctx context.Context, projectID, agentType string) (*api.ContainerConfig, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}

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
		cfg.NetworkMode = api.NetworkMode(project.NetworkMode)
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
		var mounts []api.Mount
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
	if project.EnabledRuntimes != "" {
		cfg.EnabledRuntimes = splitCSV(project.EnabledRuntimes)
	}

	return cfg, nil
}

// UpdateContainer updates a project's container configuration. If only
// lightweight settings changed (name, skipPermissions, costBudget), the
// container is updated in-place without recreation. Otherwise the container
// is fully recreated with the new configuration.
func (s *Service) UpdateContainer(ctx context.Context, projectID, agentType string, req api.CreateContainerRequest) (*ContainerResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}

	if needsRecreation(project, req) {
		return s.recreateContainer(ctx, project, req)
	}
	return s.updateContainerSettings(ctx, project, req)
}

// updateContainerSettings applies lightweight setting changes (name,
// skipPermissions, costBudget, allowedDomains) without recreating the
// container. Domain changes are hot-reloaded via docker exec on running
// restricted-mode containers.
func (s *Service) updateContainerSettings(ctx context.Context, project *db.ProjectRow, req api.CreateContainerRequest) (*ContainerResult, error) {
	containerName := effectiveContainerName(project)

	// Rename the Docker container if the name changed.
	oldContainerName := containerName
	if req.Name != "" && req.Name != containerName {
		if err := s.docker.RenameContainer(ctx, project.ContainerID, req.Name); err != nil {
			return nil, fmt.Errorf("renaming container: %w", err)
		}
		containerName = req.Name

		// Restart lifecycle watchers with the new container name.
		s.StopSessionWatcher(project.ProjectID, project.AgentType)
		if s.eventWatcher != nil {
			s.eventWatcher.CleanupContainerDir(oldContainerName)
			s.eventWatcher.WatchContainerDir(containerName)
		}
		if s.store != nil {
			s.store.RemoveContainer(oldContainerName)
		}
		s.startProjectWatcher(project.ProjectID, containerName, project.AgentType)
	}

	// Hot-reload allowed domains if they changed on a restricted-mode container.
	// Best-effort: if the exec fails (e.g. container stopped), the DB is still
	// updated so the correct domains apply on next container start/recreation.
	newDomains := strings.Join(req.AllowedDomains, ",")
	existingMode := api.NetworkMode(project.NetworkMode)
	if existingMode == "" {
		existingMode = api.NetworkModeFull
	}
	if newDomains != project.AllowedDomains && existingMode == api.NetworkModeRestricted {
		if err := s.docker.ReloadAllowedDomains(ctx, project.ContainerID, req.AllowedDomains); err != nil {
			slog.Warn("failed to hot-reload domains (container may be stopped)", "err", err)
		}
	}

	if err := s.db.UpdateProjectSettings(
		project.ProjectID,
		project.AgentType,
		req.Name,
		containerName,
		req.SkipPermissions,
		req.CostBudget,
		newDomains,
	); err != nil {
		return nil, fmt.Errorf("updating project settings: %w", err)
	}

	s.auditContainerUpdate(project, req, "settings", containerName)

	return &ContainerResult{
		ContainerID: project.ContainerID,
		Name:        containerName,
		ProjectID:   project.ProjectID,
	}, nil
}

// recreateContainer replaces the container with a new one using the full
// updated configuration.
func (s *Service) recreateContainer(ctx context.Context, project *db.ProjectRow, req api.CreateContainerRequest) (*ContainerResult, error) {
	row, err := projectRowFromRequest(req)
	if err != nil {
		return nil, err
	}

	// Resolve enabled access items into env vars and mounts.
	if err := s.ResolveAccessItemsForContainer(&req); err != nil {
		return nil, fmt.Errorf("resolving access items: %w", err)
	}

	// Merge runtime domains into the allowed list for restricted mode.
	mergeRuntimeDomains(&req)

	// Update OriginalMounts to include access item mounts (see CreateContainer).
	if len(req.Mounts) > 0 {
		if data, err := json.Marshal(req.Mounts); err == nil {
			row.OriginalMounts = data
		}
	}

	// Stop lifecycle watchers for the old container before recreation.
	oldContainerName := effectiveContainerName(project)
	s.StopSessionWatcher(project.ProjectID, project.AgentType)
	if s.eventWatcher != nil {
		s.eventWatcher.CleanupContainerDir(oldContainerName)
	}
	if s.store != nil {
		s.store.RemoveContainer(oldContainerName)
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

	// Start lifecycle watchers for the new container.
	if s.eventWatcher != nil {
		s.eventWatcher.WatchContainerDir(req.Name)
	}
	s.startProjectWatcher(row.ProjectID, req.Name, string(req.AgentType))

	s.auditContainerUpdate(project, req, "recreated", req.Name)

	return &ContainerResult{ContainerID: newID, Name: req.Name, ProjectID: row.ProjectID, Recreated: true}, nil
}

// needsRecreation reports whether the requested configuration differs from
// the current project in ways that require container recreation. Lightweight
// fields (Name, SkipPermissions, CostBudget) can be updated in-place.
func needsRecreation(project *db.ProjectRow, req api.CreateContainerRequest) bool {
	if req.Image != "" && req.Image != project.Image {
		return true
	}
	if req.ProjectPath != project.HostPath {
		return true
	}

	reqAgent := string(req.AgentType)
	if reqAgent == "" {
		reqAgent = string(agent.DefaultType)
	}
	existingAgent := project.AgentType
	if existingAgent == "" {
		existingAgent = string(agent.DefaultType)
	}
	if reqAgent != existingAgent {
		return true
	}

	reqNetwork := req.NetworkMode
	if reqNetwork == "" {
		reqNetwork = api.NetworkModeFull
	}
	existingNetwork := api.NetworkMode(project.NetworkMode)
	if existingNetwork == "" {
		existingNetwork = api.NetworkModeFull
	}
	if reqNetwork != existingNetwork {
		return true
	}

	// AllowedDomains is NOT checked here — domain changes are hot-reloaded
	// via docker exec (ReloadAllowedDomains) in the light update path.

	if !stringSlicesEqual(req.EnabledAccessItems, splitCSV(project.EnabledAccessItems)) {
		return true
	}

	if !stringSlicesEqual(req.EnabledRuntimes, splitCSV(project.EnabledRuntimes)) {
		return true
	}

	if !envVarsEqual(req.EnvVars, project.EnvVars) {
		return true
	}

	if !mountsEqual(req.Mounts, project.Mounts) {
		return true
	}

	return false
}

// envVarsEqual compares requested env vars against the JSON-encoded DB value.
func envVarsEqual(reqVars map[string]string, dbVars json.RawMessage) bool {
	var existing map[string]string
	if len(dbVars) > 0 {
		if err := json.Unmarshal(dbVars, &existing); err != nil {
			return false
		}
	}

	if len(reqVars) == 0 && len(existing) == 0 {
		return true
	}
	if len(reqVars) != len(existing) {
		return false
	}
	for k, v := range reqVars {
		if existing[k] != v {
			return false
		}
	}
	return true
}

// mountsEqual compares requested mounts against the JSON-encoded DB value.
func mountsEqual(reqMounts []api.Mount, dbMounts json.RawMessage) bool {
	var existing []api.Mount
	if len(dbMounts) > 0 {
		if err := json.Unmarshal(dbMounts, &existing); err != nil {
			return false
		}
	}

	if len(reqMounts) == 0 && len(existing) == 0 {
		return true
	}
	if len(reqMounts) != len(existing) {
		return false
	}
	for i := range reqMounts {
		if reqMounts[i] != existing[i] {
			return false
		}
	}
	return true
}

// stringSlicesEqual compares two string slices for equality.
func stringSlicesEqual(a, b []string) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// containerChange records a single field that changed between old and new values.
type containerChange struct {
	Field string `json:"field"`
	From  any    `json:"from"`
	To    any    `json:"to"`
}

// auditContainerUpdate writes a container_updated audit event with the
// changes between the old project state and the new request.
func (s *Service) auditContainerUpdate(old *db.ProjectRow, req api.CreateContainerRequest, method, containerName string) {
	var changes []containerChange

	if req.Name != "" && req.Name != old.Name {
		changes = append(changes, containerChange{"name", old.Name, req.Name})
	}
	if req.SkipPermissions != old.SkipPermissions {
		changes = append(changes, containerChange{"skipPermissions", old.SkipPermissions, req.SkipPermissions})
	}
	if req.CostBudget != old.CostBudget {
		changes = append(changes, containerChange{"costBudget", old.CostBudget, req.CostBudget})
	}
	newDomains := strings.Join(req.AllowedDomains, ",")
	if newDomains != old.AllowedDomains {
		changes = append(changes, containerChange{"allowedDomains", splitCSV(old.AllowedDomains), req.AllowedDomains})
	}
	if req.Image != "" && req.Image != old.Image {
		changes = append(changes, containerChange{"image", old.Image, req.Image})
	}
	if !stringSlicesEqual(req.EnabledRuntimes, splitCSV(old.EnabledRuntimes)) {
		changes = append(changes, containerChange{"enabledRuntimes", splitCSV(old.EnabledRuntimes), req.EnabledRuntimes})
	}
	reqNetwork := string(req.NetworkMode)
	if reqNetwork != "" && reqNetwork != old.NetworkMode {
		changes = append(changes, containerChange{"networkMode", old.NetworkMode, reqNetwork})
	}

	if len(changes) == 0 {
		return
	}

	data, _ := json.Marshal(map[string]any{
		"method":  method,
		"changes": changes,
	})

	s.audit.Write(db.Entry{
		Source:        db.SourceBackend,
		Level:         db.LevelInfo,
		ProjectID:     old.ProjectID,
		AgentType:     old.AgentType,
		ContainerName: containerName,
		Event:         "container_updated",
		Message:       fmt.Sprintf("container updated (%s): %d change(s)", method, len(changes)),
		Data:          data,
	})
}

// ValidateContainer checks whether a container has the required Warden
// terminal infrastructure installed.
func (s *Service) ValidateContainer(ctx context.Context, projectID, agentType string) (*ValidateContainerResult, error) {
	project, err := s.resolveProject(projectID, agentType)
	if err != nil {
		return nil, err
	}

	valid, missing, err := s.docker.ValidateInfrastructure(ctx, project.ContainerID)
	if err != nil {
		return nil, err
	}
	return &ValidateContainerResult{Valid: valid, Missing: missing}, nil
}

// projectRowFromRequest converts a CreateContainerRequest to a ProjectRow
// for database persistence. Computes the deterministic ProjectID from the
// host path.
func projectRowFromRequest(req api.CreateContainerRequest) (db.ProjectRow, error) {
	projectID, err := engine.ProjectID(req.ProjectPath)
	if err != nil {
		return db.ProjectRow{}, fmt.Errorf("computing project ID: %w", err)
	}

	agentType := string(req.AgentType)
	if agentType == "" {
		agentType = string(agent.DefaultType)
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
	if len(req.EnabledRuntimes) > 0 {
		row.EnabledRuntimes = strings.Join(req.EnabledRuntimes, ",")
	}

	return row, nil
}

// mergeRuntimeDomains adds runtime-contributed network domains to the
// request's allowed domain list when network mode is restricted. The DB
// stores user-entered domains only (via projectRowFromRequest which runs
// before this); merged domains are only passed to the Docker engine.
func mergeRuntimeDomains(req *api.CreateContainerRequest) {
	if req.NetworkMode != api.NetworkModeRestricted || len(req.EnabledRuntimes) == 0 {
		return
	}
	runtimeDomains := runtimes.DomainsForRuntimes(req.EnabledRuntimes)
	existing := make(map[string]bool, len(req.AllowedDomains))
	for _, d := range req.AllowedDomains {
		existing[d] = true
	}
	for _, d := range runtimeDomains {
		if !existing[d] {
			req.AllowedDomains = append(req.AllowedDomains, d)
		}
	}
}

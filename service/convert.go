package service

import (
	"github.com/thesimonho/warden/api"
	"github.com/thesimonho/warden/engine"
)

// projectResponseFromEngine converts an engine.Project to an api.ProjectResponse.
// This is the single conversion point between the domain type and the HTTP
// contract type — if the HTTP response needs to diverge from the domain
// model, change this function.
func projectResponseFromEngine(p engine.Project) api.ProjectResponse {
	return api.ProjectResponse{
		ProjectID:           p.ProjectID,
		ID:                  p.ID,
		Name:                p.Name,
		HostPath:            p.HostPath,
		HasContainer:        p.HasContainer,
		Type:                p.Type,
		Image:               p.Image,
		OS:                  p.OS,
		CreatedAt:           p.CreatedAt,
		SSHPort:             p.SSHPort,
		State:               p.State,
		Status:              p.Status,
		AgentStatus:         string(p.AgentStatus),
		NeedsInput:          p.NeedsInput,
		NotificationType:    string(p.NotificationType),
		ActiveWorktreeCount: p.ActiveWorktreeCount,
		TotalCost:           p.TotalCost,
		IsEstimatedCost:     p.IsEstimatedCost,
		CostBudget:          p.CostBudget,
		IsGitRepo:           p.IsGitRepo,
		AgentType:           p.AgentType,
		SkipPermissions:     p.SkipPermissions,
		MountedDir:          p.MountedDir,
		WorkspaceDir:        p.WorkspaceDir,
		NetworkMode:         p.NetworkMode,
		AllowedDomains:      p.AllowedDomains,
		AgentVersion:        p.AgentVersion,
	}
}

// projectResponsesFromEngine converts a slice of engine.Project to
// api.ProjectResponse.
func projectResponsesFromEngine(projects []engine.Project) []api.ProjectResponse {
	responses := make([]api.ProjectResponse, len(projects))
	for i, p := range projects {
		responses[i] = projectResponseFromEngine(p)
	}
	return responses
}

// worktreeResult builds a WorktreeResult with the given state.
func worktreeResult(worktreeID, projectID string, state engine.WorktreeState) *api.WorktreeResult {
	return &api.WorktreeResult{
		WorktreeID: worktreeID,
		ProjectID:  projectID,
		State:      string(state),
	}
}

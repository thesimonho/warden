package server

// OpenAPI response and request types for handlers that use anonymous structs.
// These types exist solely for swag annotation references — the handlers
// themselves continue to use inline structs/maps. The linter correctly flags
// them as unused in Go code; they are only consumed by the swag parser.

// addProjectRequest is the JSON body for adding a project by container name.
type addProjectRequest struct { //nolint:unused
	// Name is the Docker container name to add.
	Name string `json:"name" example:"my-project"`
}

// createWorktreeRequest is the JSON body for creating a new worktree.
type createWorktreeRequest struct { //nolint:unused
	// Name is the worktree name (must be a valid git branch name).
	Name string `json:"name" example:"feature-auth"`
}

// revealRequest is the JSON body for revealing a directory in the file manager.
type revealRequest struct { //nolint:unused
	// Path is the absolute host directory path to open.
	Path string `json:"path" example:"/home/user/projects"`
}

// cleanupWorktreesResponse is the response from worktree cleanup.
type cleanupWorktreesResponse struct { //nolint:unused
	// Removed is the list of orphaned worktree IDs that were cleaned up.
	Removed []string `json:"removed"`
}

// validateContainerResponse is the response from container infrastructure validation.
type validateContainerResponse struct { //nolint:unused
	// Valid is true when all required infrastructure is present.
	Valid bool `json:"valid"`
	// Missing lists the infrastructure components that are not installed.
	Missing []string `json:"missing"`
}

// updateSettingsResponse is the response from updating settings.
type updateSettingsResponse struct { //nolint:unused
	// RestartRequired is true when a setting change requires a server restart.
	RestartRequired bool `json:"restartRequired"`
}

// healthResponse is the response from the health check endpoint.
type healthResponse struct { //nolint:unused
	// Status is always "ok" when the server is running.
	Status string `json:"status" example:"ok"`
}

package server

// OpenAPI response and request types for handlers that use anonymous structs.
// These types exist solely for swag annotation references — the handlers
// themselves continue to use inline structs/maps. The linter correctly flags
// them as unused in Go code; they are only consumed by the swag parser.

// revealRequest is the JSON body for revealing a directory in the file manager.
type revealRequest struct { //nolint:unused
	// Path is the absolute host directory path to open.
	Path string `json:"path" example:"/home/user/projects"`
}

// cleanupWorktreesResponse is the response from worktree cleanup.
type cleanupWorktreesResponse struct {
	// Removed is the list of orphaned worktree IDs that were cleaned up.
	Removed []string `json:"removed"`
}

// validateContainerResponse is the response from container infrastructure validation.
type validateContainerResponse struct {
	// Valid is true when all required infrastructure is present.
	Valid bool `json:"valid"`
	// Missing lists the infrastructure components that are not installed.
	Missing []string `json:"missing"`
}

// updateSettingsResponse is the response from updating settings.
type updateSettingsResponse struct {
	// RestartRequired is true when a setting change requires a server restart.
	RestartRequired bool `json:"restartRequired"`
}

// healthResponse is the response from the health check endpoint.
type healthResponse struct {
	// Status is always "ok" when the server is running.
	Status string `json:"status" example:"ok"`
	// Version is the server build version.
	Version string `json:"version" example:"v0.5.2"`
}

// shutdownResponse is the response from the shutdown endpoint.
type shutdownResponse struct {
	// Status is always "shutting down".
	Status string `json:"status" example:"shutting down"`
}

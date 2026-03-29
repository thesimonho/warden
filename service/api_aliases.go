package service

import "github.com/thesimonho/warden/api"

// Type aliases for backward compatibility. Service methods return these
// types; the canonical definitions live in the api package.
type (
	ProjectResult           = api.ProjectResult
	WorktreeResult          = api.WorktreeResult
	ContainerResult         = api.ContainerResult
	ValidateContainerResult = api.ValidateContainerResult
	SettingsResponse        = api.SettingsResponse
	UpdateSettingsRequest   = api.UpdateSettingsRequest
	UpdateSettingsResult    = api.UpdateSettingsResult
	PostAuditEventRequest   = api.PostAuditEventRequest
	DefaultMount            = api.DefaultMount
	DefaultEnvVar           = api.DefaultEnvVar
	DefaultsResponse        = api.DefaultsResponse
	DirEntry                = api.DirEntry
	AuditFilters            = api.AuditFilters
	AuditCategory           = api.AuditCategory
	AuditSummary            = api.AuditSummary
)

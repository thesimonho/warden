package service

import "github.com/thesimonho/warden/api"

// Type aliases for backward compatibility. Service methods return these
// types; the canonical definitions live in the api package.
type (
	ProjectResponse            = api.ProjectResponse
	AddProjectRequest          = api.AddProjectRequest
	AddProjectResponse         = api.AddProjectResponse
	BatchProjectAction         = api.BatchProjectAction
	BatchProjectRef            = api.BatchProjectRef
	BatchProjectRequest        = api.BatchProjectRequest
	BatchProjectResponse       = api.BatchProjectResponse
	BatchProjectResult         = api.BatchProjectResult
	CreateWorktreeRequest      = api.CreateWorktreeRequest
	WorktreeInputRequest       = api.WorktreeInputRequest
	ProjectCostsResponse       = api.ProjectCostsResponse
	SessionCostEntry           = api.SessionCostEntry
	BudgetStatusResponse       = api.BudgetStatusResponse
	BudgetSource               = api.BudgetSource
	ProjectResult              = api.ProjectResult
	WorktreeResult             = api.WorktreeResult
	ContainerResult            = api.ContainerResult
	ValidateContainerResult    = api.ValidateContainerResult
	SettingsResponse           = api.SettingsResponse
	UpdateSettingsRequest      = api.UpdateSettingsRequest
	UpdateSettingsResult       = api.UpdateSettingsResult
	PostAuditEventRequest      = api.PostAuditEventRequest
	AuditLogMode               = api.AuditLogMode
	DefaultMount               = api.DefaultMount
	DefaultEnvVar              = api.DefaultEnvVar
	DefaultsResponse           = api.DefaultsResponse
	DirEntry                   = api.DirEntry
	AuditEntry                 = api.AuditEntry
	AuditSource                = api.AuditSource
	AuditLevel                 = api.AuditLevel
	AuditFilters               = api.AuditFilters
	AuditCategory              = api.AuditCategory
	AuditSummary               = api.AuditSummary
	ClipboardUploadResponse    = api.ClipboardUploadResponse
	CreateContainerRequest     = api.CreateContainerRequest
	ContainerConfig            = api.ContainerConfig
	RuntimeDefault             = api.RuntimeDefault
	Mount                      = api.Mount
	NetworkMode                = api.NetworkMode
	DiffResponse               = api.DiffResponse
	DiffFileSummary            = api.DiffFileSummary
	ToolCount                  = api.ToolCount
	TimeRange                  = api.TimeRange
	ProjectTemplate            = api.ProjectTemplate
	AgentTemplateOverride      = api.AgentTemplateOverride
)

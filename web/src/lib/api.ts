/**
 * Warden HTTP API client — reference implementation for web frontends.
 *
 * If you're building a web application against the Warden API, copy the
 * patterns in this file. Every function demonstrates how to call a specific
 * endpoint and type the response. Key patterns:
 *
 * - **Centralized error handling**: `apiFetch` wraps `fetch`, checks the
 *   response status, and extracts the `{ error }` body the Go server returns.
 *   All callers get consistent error messages without per-call try/catch.
 *
 * - **Response typing**: Each function casts the parsed JSON to a known type
 *   via `response.json() as Promise<T>`. The types in `types.ts` mirror the
 *   Go structs — keep them in sync when the API changes.
 *
 * - **Request bodies**: POST/PUT calls set `Content-Type: application/json`
 *   and pass `JSON.stringify(body)`. The Go server expects JSON for all
 *   mutation endpoints.
 *
 * - **Query parameters**: GET endpoints with filters (e.g. `fetchAuditLog`)
 *   build a `URLSearchParams` and append it to the URL. Only non-empty
 *   values are included.
 *
 * For the Go HTTP client equivalent, see the `client/` package.
 * For SSE events, see `use-event-source.ts`.
 * For terminal WebSocket, see `use-terminal.ts`.
 *
 * @module
 */
import type {
  Project,
  ContainerConfig,
  Worktree,
  WorktreeResult,
  ProjectResult,
  ContainerResult,
  CreateContainerRequest,
  DiffResponse,
  DirEntry,
  AuditLogEntry,
  AuditLogLevel,
  RuntimeInfo,
  ServerSettings,
  AuditFilters,
  AuditSummary,
} from '@/lib/types'

/** Base URL for API requests. Empty string since Vite proxies /api to the backend. */
const API_BASE = ''

/**
 * Error thrown by `apiFetch` for non-ok responses.
 *
 * The `code` field contains a machine-readable error code (e.g. "NAME_TAKEN",
 * "REQUIRED_FIELD") for programmatic error handling. Match on `code` instead
 * of parsing the human-readable `message`.
 */
export class ApiError extends Error {
  readonly code: string
  readonly status: number

  constructor(status: number, code: string, message: string) {
    super(message)
    this.name = 'ApiError'
    this.status = status
    this.code = code
  }
}

/**
 * Performs a fetch request and throws on non-ok responses.
 *
 * This is the single point of error handling for all API calls. The Go server
 * returns errors as `{ "error": "message", "code": "CODE" }` — this function
 * extracts both and throws an `ApiError` so callers can display the message
 * directly or branch on the code.
 *
 * @param url - The URL to fetch.
 * @param options - Optional fetch init options.
 * @returns The raw Response object.
 * @throws ApiError with the server's error code and message, or the HTTP status text.
 */
async function apiFetch(url: string, options?: RequestInit): Promise<Response> {
  const response = await fetch(`${API_BASE}${url}`, options)

  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    let code = 'UNKNOWN'
    try {
      const body = (await response.json()) as { error?: string; code?: string }
      if (body.error) {
        message = body.error
      }
      if (body.code) {
        code = body.code
      }
    } catch {
      // Response body wasn't JSON, use status text
    }
    throw new ApiError(response.status, code, message)
  }

  return response
}

/**
 * Fetches all projects from the API.
 *
 * @returns An array of projects.
 */
export async function fetchProjects(): Promise<Project[]> {
  const response = await apiFetch('/api/v1/projects')
  return response.json() as Promise<Project[]>
}

/**
 * Stops a running project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The project name and container ID.
 */
export async function stopProject(projectId: string): Promise<ProjectResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/stop`, { method: 'POST' })
  return response.json() as Promise<ProjectResult>
}

/**
 * Restarts a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The project name and container ID.
 */
export async function restartProject(projectId: string): Promise<ProjectResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/restart`, { method: 'POST' })
  return response.json() as Promise<ProjectResult>
}

/**
 * Fetches all worktrees for a given project with their terminal state.
 *
 * @param projectId - The project ID to fetch worktrees for.
 * @returns An array of worktrees belonging to the project.
 */
export async function fetchWorktrees(projectId: string): Promise<Worktree[]> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees`)
  return response.json() as Promise<Worktree[]>
}

/**
 * Creates a new git worktree and connects a terminal to it.
 *
 * @param projectId - The project to create the worktree in.
 * @param name - The name for the new worktree.
 * @returns The worktree result with worktree and project IDs.
 */
export async function createWorktree(projectId: string, name: string): Promise<WorktreeResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  return response.json() as Promise<WorktreeResult>
}

/**
 * Connects a terminal to a worktree, starting Claude Code.
 *
 * @param projectId - The project the worktree belongs to.
 * @param worktreeId - The worktree ID to connect.
 * @returns The worktree result with worktree and project IDs.
 */
export async function connectTerminal(
  projectId: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees/${worktreeId}/connect`, {
    method: 'POST',
  })
  return response.json() as Promise<WorktreeResult>
}

/**
 * Disconnects the terminal viewer from a worktree.
 * The abduco session (and Claude/bash) continues running in the background.
 *
 * @param projectId - The project the worktree belongs to.
 * @param worktreeId - The worktree ID to disconnect.
 * @returns The worktree result with worktree and project IDs.
 */
export async function disconnectTerminal(
  projectId: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(
    `/api/v1/projects/${projectId}/worktrees/${worktreeId}/disconnect`,
    { method: 'POST' },
  )
  return response.json() as Promise<WorktreeResult>
}

/**
 * Kills abduco and all child processes for a worktree.
 * This is destructive — the terminal session is destroyed and cannot be reconnected.
 *
 * @param projectId - The project the worktree belongs to.
 * @param worktreeId - The worktree ID to kill.
 * @returns The worktree result with worktree and project IDs.
 */
export async function killWorktreeProcess(
  projectId: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees/${worktreeId}/kill`, {
    method: 'POST',
  })
  return response.json() as Promise<WorktreeResult>
}

/**
 * Removes a worktree entirely: kills processes, runs `git worktree remove`,
 * and cleans up tracking state. Cannot remove the main worktree.
 *
 * @param projectId - The project the worktree belongs to.
 * @param worktreeId - The worktree ID to remove.
 * @returns The worktree result with worktree and project IDs.
 */
export async function removeWorktree(
  projectId: string,
  worktreeId: string,
): Promise<WorktreeResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees/${worktreeId}`, {
    method: 'DELETE',
  })
  return response.json() as Promise<WorktreeResult>
}

/** Result of cleaning up orphaned worktree directories. */
interface CleanupWorktreesResult {
  removed: string[] | null
}

/**
 * Removes orphaned worktree directories that exist on disk but are no longer tracked by git.
 *
 * @param projectId - The project whose container to clean up.
 * @returns The list of removed worktree IDs.
 */
export async function cleanupWorktrees(projectId: string): Promise<CleanupWorktreesResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees/cleanup`, {
    method: 'POST',
  })
  return response.json() as Promise<CleanupWorktreesResult>
}

/**
 * Fetches uncommitted changes (tracked + untracked) for a worktree.
 *
 * @param projectId - Container ID.
 * @param worktreeId - Worktree ID.
 * @returns Diff response with per-file stats and unified diff.
 */
export async function fetchWorktreeDiff(
  projectId: string,
  worktreeId: string,
): Promise<DiffResponse> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/worktrees/${worktreeId}/diff`)
  return response.json() as Promise<DiffResponse>
}

/** Result of validating a container's Warden infrastructure. */
interface ValidateContainerResult {
  valid: boolean
  missing: string[] | null
}

/**
 * Validates whether a project's container has the required Warden terminal infrastructure.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns Whether the container is valid and which binaries are missing.
 */
export async function validateContainer(projectId: string): Promise<ValidateContainerResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/container/validate`)
  return response.json() as Promise<ValidateContainerResult>
}

/**
 * Creates a new container for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param req - The container creation request.
 * @returns The container ID and name.
 */
export async function createContainer(
  projectId: string,
  req: CreateContainerRequest,
): Promise<ContainerResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/container`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })

  return response.json() as Promise<ContainerResult>
}

/**
 * Computes the host filesystem path for a worktree by stripping the
 * container-side workspace prefix and prepending the host mount dir.
 *
 * Falls back to `mountedDir + containerPath` when `containerPath` does not
 * start with `workspaceDir` (should not happen in practice — all worktree
 * paths live under the workspace mount).
 */
export function worktreeHostPath(
  mountedDir: string,
  containerPath: string,
  workspaceDir: string,
): string {
  const relative = containerPath.startsWith(workspaceDir)
    ? containerPath.slice(workspaceDir.length)
    : containerPath
  return mountedDir + relative
}

/**
 * Opens a directory in the host's file manager (Finder, Explorer, etc.).
 *
 * @param path - Absolute host path to reveal.
 */
export async function revealInFileManager(path: string): Promise<void> {
  await apiFetch('/api/v1/filesystem/reveal', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ path }),
  })
}

/**
 * Lists filesystem entries at the given path for the browser.
 *
 * @param dirPath - Absolute path to list entries in.
 * @param includeFiles - When true, returns files alongside directories.
 * @returns An array of filesystem entries.
 */
export async function listDirectories(dirPath: string, includeFiles = false): Promise<DirEntry[]> {
  const params = new URLSearchParams({ path: dirPath })
  if (includeFiles) {
    params.set('mode', 'file')
  }
  const response = await apiFetch(`/api/v1/filesystem/directories?${params.toString()}`)
  return response.json() as Promise<DirEntry[]>
}

/**
 * Adds a project to the dashboard.
 *
 * @param name - The project name.
 * @param projectPath - Absolute host path for the project directory.
 * @returns The project name.
 */
export async function addProject(name: string, projectPath: string): Promise<ProjectResult> {
  const response = await apiFetch('/api/v1/projects', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, projectPath }),
  })
  return response.json() as Promise<ProjectResult>
}

/**
 * Removes a project from the dashboard.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The project name.
 */
export async function removeProject(projectId: string): Promise<ProjectResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}`, {
    method: 'DELETE',
  })
  return response.json() as Promise<ProjectResult>
}

/**
 * Resets all cost history for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 */
export async function resetProjectCosts(projectId: string): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/costs`, { method: 'DELETE' })
}

/**
 * Purges all audit events for a project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The number of deleted events.
 */
export async function purgeProjectAudit(projectId: string): Promise<{ deleted: number }> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/audit`, { method: 'DELETE' })
  return response.json() as Promise<{ deleted: number }>
}

/**
 * Deletes a project's container.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The container ID and name.
 */
export async function deleteContainer(projectId: string): Promise<ContainerResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/container`, { method: 'DELETE' })
  return response.json() as Promise<ContainerResult>
}

/**
 * Fetches the editable configuration of a project's container.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @returns The container's configuration.
 */
export async function fetchContainerConfig(projectId: string): Promise<ContainerConfig> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/container/config`)
  return response.json() as Promise<ContainerConfig>
}

/**
 * Recreates a project's container with updated configuration.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 * @param req - The new container configuration.
 * @returns The new container ID and name.
 */
export async function updateContainer(
  projectId: string,
  req: CreateContainerRequest,
): Promise<ContainerResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/container`, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  })
  return response.json() as Promise<ContainerResult>
}

/**
 * Fetches available container runtimes and their status.
 *
 * @returns An array of runtime info objects.
 */
export async function fetchRuntimes(): Promise<RuntimeInfo[]> {
  const response = await apiFetch('/api/v1/runtimes')
  return response.json() as Promise<RuntimeInfo[]>
}

/**
 * Fetches server-side settings.
 *
 * @returns The current server settings.
 */
export async function fetchSettings(): Promise<ServerSettings> {
  const response = await apiFetch('/api/v1/settings')
  return response.json() as Promise<ServerSettings>
}

/**
 * Updates server-side settings.
 *
 * @param settings - The settings to update.
 * @returns Whether a server restart is required.
 */
export async function updateSettings(
  settings: Partial<ServerSettings>,
): Promise<{ restartRequired: boolean }> {
  const response = await apiFetch('/api/v1/settings', {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(settings),
  })
  return response.json() as Promise<{ restartRequired: boolean }>
}

/** Server-resolved default values for the create container form. */
export interface Defaults {
  homeDir: string
  containerHomeDir: string
  mounts: Array<{ hostPath: string; containerPath: string; readOnly: boolean }>
  envVars?: Array<{ key: string; value: string }>
}

/**
 * Fetches server-resolved default values (e.g. the host's ~/.claude path).
 *
 * @returns Default configuration values resolved on the server.
 */
export async function fetchDefaults(): Promise<Defaults> {
  const response = await apiFetch('/api/v1/defaults')
  return response.json() as Promise<Defaults>
}

// --- Audit Log ---

/** Converts audit filter fields to URLSearchParams. */
function auditFiltersToParams(filters?: AuditFilters): URLSearchParams {
  const params = new URLSearchParams()
  if (filters?.projectId) params.set('projectId', filters.projectId)
  if (filters?.worktree) params.set('worktree', filters.worktree)
  if (filters?.source) params.set('source', filters.source)
  if (filters?.level) params.set('level', filters.level)
  if (filters?.since) params.set('since', filters.since)
  if (filters?.until) params.set('until', filters.until)
  return params
}

/**
 * Fetches audit-relevant events from the server with optional filters.
 *
 * @param filters - Optional filters for container, worktree, category, time range.
 * @returns Array of event log entries matching the audit criteria.
 */
export async function fetchAuditLog(filters?: AuditFilters): Promise<AuditLogEntry[]> {
  const params = auditFiltersToParams(filters)
  if (filters?.category) params.set('category', filters.category)
  if (filters?.limit) params.set('limit', String(filters.limit))
  if (filters?.offset) params.set('offset', String(filters.offset))
  const query = params.toString()
  const path = query ? `/api/v1/audit?${query}` : '/api/v1/audit'
  const response = await apiFetch(path)
  return response.json() as Promise<AuditLogEntry[]>
}

/**
 * Fetches aggregate audit statistics.
 *
 * @param filters - Optional filters for container, worktree, time range.
 * @returns Summary with session, tool, prompt counts and top tools.
 */
export async function fetchAuditSummary(filters?: AuditFilters): Promise<AuditSummary> {
  const params = auditFiltersToParams(filters)
  const query = params.toString()
  const path = query ? `/api/v1/audit/summary?${query}` : '/api/v1/audit/summary'
  const response = await apiFetch(path)
  return response.json() as Promise<AuditSummary>
}

/**
 * Builds the URL for downloading audit log exports.
 *
 * @param format - Export format: 'csv' or 'json'.
 * @param filters - Optional filters for container, worktree, category, time range.
 * @returns URL string for the export endpoint.
 */
export function auditExportUrl(format: 'csv' | 'json', filters?: AuditFilters): string {
  const params = auditFiltersToParams(filters)
  params.set('format', format)
  if (filters?.category) params.set('category', filters.category)
  return `${API_BASE}/api/v1/audit/export?${params.toString()}`
}

/** Fetches distinct project (container) names from the audit log. */
export async function fetchAuditProjects(): Promise<string[]> {
  const response = await apiFetch('/api/v1/audit/projects')
  return response.json() as Promise<string[]>
}

/** Posts a frontend event to the audit log. */
export async function postAuditEvent(params: {
  event: string
  level?: AuditLogLevel
  message?: string
  attrs?: Record<string, unknown>
}): Promise<void> {
  await apiFetch('/api/v1/audit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(params),
  })
}

/** Deletes audit events matching the given filters. */
export async function deleteAuditEvents(filters?: AuditFilters): Promise<void> {
  const params = auditFiltersToParams(filters)
  const query = params.toString()
  const path = query ? `/api/v1/audit?${query}` : '/api/v1/audit'
  await apiFetch(path, { method: 'DELETE' })
}

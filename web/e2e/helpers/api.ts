/**
 * Direct API wrappers for E2E test setup and teardown.
 *
 * These bypass the UI and call the Warden API directly via fetch.
 * Used by fixtures to create/destroy test projects without coupling
 * tests to UI form interactions.
 */

/**
 * Resolves the API base URL by probing available servers.
 * Prefers Vite dev server (:5173) since it always has the latest code.
 */
let _resolvedBaseURL: string | undefined
export async function getBaseURL(): Promise<string> {
  if (_resolvedBaseURL) return _resolvedBaseURL
  for (const candidate of ['http://localhost:5173', 'http://localhost:8090']) {
    try {
      const response = await fetch(`${candidate}/api/v1/health`, {
        signal: AbortSignal.timeout(2000),
      })
      if (!response.ok) continue

      // Validate the response is JSON, not an SPA fallback HTML page.
      // When the Go backend is down, Vite returns index.html with 200
      // for any route — including /api/v1/health.
      const body = await response.json() as { status?: string }
      if (body.status !== 'ok') continue

      _resolvedBaseURL = candidate
      return candidate
    } catch {
      /* not reachable or not JSON — try next */
    }
  }
  _resolvedBaseURL = 'http://localhost:8090'
  return _resolvedBaseURL
}

/** Performs a fetch against the Warden API, throwing on non-ok responses. */
async function apiFetch(path: string, options?: RequestInit): Promise<Response> {
  const baseURL = await getBaseURL()
  const response = await fetch(`${baseURL}${path}`, options)
  if (!response.ok) {
    let message = `${response.status} ${response.statusText}`
    try {
      const body = (await response.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      /* not JSON */
    }
    throw new Error(`API ${options?.method ?? 'GET'} ${path}: ${message}`)
  }
  return response
}

/** Project as returned by the API. */
export interface ApiProject {
  /** Deterministic project ID (12-char hex hash of host path). */
  projectId: string
  /** Agent type (e.g. "claude-code", "codex"). */
  agentType: string
  /** Docker container ID (empty when no container exists). */
  id: string
  name: string
  state: string
  hasContainer: boolean
  activeWorktreeCount: number
}

/** Worktree as returned by the API. */
export interface ApiWorktree {
  id: string
  projectId: string
  state: string
  branch?: string
}

/** Runtime info from the API. */
export interface ApiRuntime {
  name: string
  available: boolean
  socketPath: string
  version?: string
}

/** Result of validating a container's Warden infrastructure. */
export interface ValidateResult {
  valid: boolean
  missing: string[] | null
}

/** Validates container infrastructure (tmux, scripts). */
export async function validateContainer(projectId: string, agentType = 'claude-code'): Promise<ValidateResult> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/container/validate`)
  return response.json() as Promise<ValidateResult>
}

/** Fetches all projects. */
export async function fetchProjects(): Promise<ApiProject[]> {
  const response = await apiFetch('/api/v1/projects')
  return response.json() as Promise<ApiProject[]>
}

/** Fetches worktrees for a project. */
export async function fetchWorktrees(projectId: string, agentType = 'claude-code'): Promise<ApiWorktree[]> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/worktrees`)
  return response.json() as Promise<ApiWorktree[]>
}

/** Fetches available container runtimes. */
export async function fetchRuntimes(): Promise<ApiRuntime[]> {
  const response = await apiFetch('/api/v1/runtimes')
  return response.json() as Promise<ApiRuntime[]>
}

/** Project result from add/remove operations. */
interface ProjectResult {
  projectId: string
  name: string
  containerId?: string
}

/** Container result from create/delete operations. */
interface ContainerResult {
  containerId: string
  name: string
  projectId: string
}

/**
 * Creates a test project: registers the directory then creates a container.
 *
 * Two-step flow matching the current API:
 * 1. POST /api/v1/projects — register the host directory
 * 2. POST /api/v1/projects/{projectId}/container — create the container
 */
export async function createTestProject(
  name: string,
  projectPath: string,
  options?: {
    skipPermissions?: boolean
    image?: string
    agentType?: string
    enabledAccessItems?: string[]
  },
): Promise<{ projectId: string; containerId: string; name: string }> {
  /* Step 1: Register the project directory. */
  const addResponse = await apiFetch('/api/v1/projects', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name, projectPath }),
  })
  const projectResult = (await addResponse.json()) as ProjectResult

  /* Step 2: Create a container for the project. */
  const agentType = options?.agentType ?? 'claude-code'
  const createResponse = await apiFetch(
    `/api/v1/projects/${projectResult.projectId}/${agentType}/container`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name,
        image: options?.image ?? 'warden-e2e:local',
        projectPath,
        agentType: options?.agentType,
        skipPermissions: options?.skipPermissions ?? true,
        enabledAccessItems: options?.enabledAccessItems,
      }),
    },
  )
  const containerResult = (await createResponse.json()) as ContainerResult

  return {
    projectId: projectResult.projectId,
    containerId: containerResult.containerId,
    name: containerResult.name,
  }
}

/**
 * Removes a test project: deletes the container then unregisters the project.
 *
 * @param projectId - The stable project ID (12-char hex hash).
 */
export async function removeTestProject(projectId: string, agentType = 'claude-code'): Promise<void> {
  try {
    await apiFetch(`/api/v1/projects/${projectId}/${agentType}/container`, { method: 'DELETE' })
  } catch {
    /* Container may already be gone. */
  }
  // Purge audit events and costs before removing the project so test
  // data doesn't leak into the user's real DB when reusing a dev server.
  try {
    await apiFetch(`/api/v1/projects/${projectId}/${agentType}/audit`, { method: 'DELETE' })
  } catch { /* best-effort */ }
  try {
    await apiFetch(`/api/v1/projects/${projectId}/${agentType}/costs`, { method: 'DELETE' })
  } catch { /* best-effort */ }
  try {
    await apiFetch(`/api/v1/projects/${projectId}/${agentType}`, { method: 'DELETE' })
  } catch {
    /* Project may already be removed. */
  }
}

/** Stops a project container. */
export async function stopProject(projectId: string, agentType = 'claude-code'): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/${agentType}/stop`, { method: 'POST' })
}

/** Restarts a project container. */
export async function restartProject(projectId: string, agentType = 'claude-code'): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/${agentType}/restart`, { method: 'POST' })
}

/** Connects a terminal to a worktree. */
export async function connectTerminal(
  projectId: string,
  worktreeId: string,
  agentType = 'claude-code',
): Promise<{ worktreeId: string }> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}/connect`, {
    method: 'POST',
  })
  return response.json() as Promise<{ worktreeId: string }>
}

/** Creates a new worktree and connects a terminal to it. */
export async function createWorktree(
  projectId: string,
  name: string,
  agentType = 'claude-code',
): Promise<{ worktreeId: string }> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/worktrees`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  })
  return response.json() as Promise<{ worktreeId: string }>
}

/** Kills all processes for a worktree (tmux session + Claude). Fully stops the worktree. */
export async function killWorktreeProcess(projectId: string, worktreeId: string, agentType = 'claude-code'): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}/kill`, { method: 'POST' })
}

/** Disconnects a terminal from a worktree. */
export async function disconnectTerminal(projectId: string, worktreeId: string, agentType = 'claude-code'): Promise<void> {
  await apiFetch(`/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}/disconnect`, {
    method: 'POST',
  })
}

/**
 * Polls until a project reaches the expected state.
 *
 * @param name - Project name to look for.
 * @param expectedState - State to wait for (e.g. "running").
 * @param timeoutMs - Max time to wait.
 * @returns The project once it reaches the expected state.
 */
export async function waitForProjectState(
  name: string,
  expectedState: string,
  timeoutMs = 60_000,
): Promise<ApiProject> {
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const projects = await fetchProjects()
    // Find the project with a container — stale DB entries from previous
    // runs may have the same name but no container (empty state).
    const project = projects.find((p) => p.name === name && p.hasContainer)
    if (project?.state === expectedState) return project
    await new Promise((r) => setTimeout(r, 2000))
  }
  throw new Error(`Project "${name}" did not reach state "${expectedState}" within ${timeoutMs}ms`)
}

/**
 * Polls until a worktree reaches the expected state.
 *
 * @param projectId - Container ID.
 * @param worktreeId - Worktree ID.
 * @param expectedState - State to wait for.
 * @param timeoutMs - Max time to wait.
 */
export async function waitForWorktreeState(
  projectId: string,
  worktreeId: string,
  expectedState: string | string[],
  timeoutMs = 30_000,
  agentType = 'claude-code',
): Promise<ApiWorktree> {
  const validStates = Array.isArray(expectedState) ? expectedState : [expectedState]
  const deadline = Date.now() + timeoutMs
  while (Date.now() < deadline) {
    const worktrees = await fetchWorktrees(projectId, agentType)
    const wt = worktrees.find((w) => w.id === worktreeId)
    if (wt && validStates.includes(wt.state)) return wt
    await new Promise((r) => setTimeout(r, 1000))
  }
  throw new Error(
    `Worktree "${worktreeId}" did not reach state "${validStates.join(' or ')}" within ${timeoutMs}ms`,
  )
}

// --- New integrator endpoints ---

/** Fetches a single project by ID. */
export async function fetchProject(
  projectId: string,
  agentType = 'claude-code',
): Promise<ApiProject> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}`)
  return response.json() as Promise<ApiProject>
}

/** Fetches a single worktree by ID. */
export async function fetchWorktree(
  projectId: string,
  worktreeId: string,
  agentType = 'claude-code',
): Promise<ApiWorktree> {
  const response = await apiFetch(
    `/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}`,
  )
  return response.json() as Promise<ApiWorktree>
}

/** Project costs response. */
export interface ApiProjectCosts {
  projectId: string
  agentType: string
  totalCost: number
  isEstimated: boolean
  sessions: Array<{
    sessionId: string
    cost: number
    isEstimated: boolean
    createdAt: string
    updatedAt: string
  }>
}

/** Fetches project costs. */
export async function fetchProjectCosts(
  projectId: string,
  agentType = 'claude-code',
): Promise<ApiProjectCosts> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/costs`)
  return response.json() as Promise<ApiProjectCosts>
}

/** Budget status response. */
export interface ApiBudgetStatus {
  projectId: string
  agentType: string
  effectiveBudget: number
  totalCost: number
  isOverBudget: boolean
  isEstimatedCost: boolean
  budgetSource: string
}

/** Fetches budget status for a project. */
export async function fetchBudgetStatus(
  projectId: string,
  agentType = 'claude-code',
): Promise<ApiBudgetStatus> {
  const response = await apiFetch(`/api/v1/projects/${projectId}/${agentType}/budget`)
  return response.json() as Promise<ApiBudgetStatus>
}

/** Posts a custom audit event. */
export async function postAuditEvent(event: {
  event: string
  source?: string
  level?: string
  message?: string
  projectId?: string
  agentType?: string
  worktree?: string
  data?: unknown
  attrs?: Record<string, unknown>
}): Promise<void> {
  await apiFetch('/api/v1/audit', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(event),
  })
}

/** Fetches audit log entries. */
export async function fetchAuditLog(params?: {
  projectId?: string
  source?: string
  category?: string
}): Promise<Array<{ event: string; source: string; projectId?: string; msg?: string }>> {
  const query = new URLSearchParams()
  if (params?.projectId) query.set('projectId', params.projectId)
  if (params?.source) query.set('source', params.source)
  if (params?.category) query.set('category', params.category)
  const qs = query.toString()
  const response = await apiFetch(`/api/v1/audit${qs ? `?${qs}` : ''}`)
  return response.json() as Promise<Array<{ event: string; source: string; projectId?: string; msg?: string }>>
}

/** Fetches the worktree diff (uncommitted changes). */
export async function fetchWorktreeDiff(
  projectId: string,
  worktreeId: string,
  agentType = 'claude-code',
): Promise<{ files: Array<{ path: string; status: string }> }> {
  const response = await apiFetch(
    `/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}/diff`,
  )
  return response.json() as Promise<{ files: Array<{ path: string; status: string }> }>
}

/** SSE event from the event stream. */
export interface SSEEvent {
  event: string
  data: Record<string, unknown>
}

/**
 * Opens an SSE connection and collects events until the timeout.
 * Returns the collected events. Used for testing SSE filtering behavior.
 */
export async function collectSSEEvents(
  options: { projectId?: string; agentType?: string; timeoutMs?: number },
): Promise<SSEEvent[]> {
  const baseURL = await getBaseURL()
  const params = new URLSearchParams()
  if (options.projectId) params.set('projectId', options.projectId)
  if (options.agentType) params.set('agentType', options.agentType)
  const qs = params.toString()
  const url = `${baseURL}/api/v1/events${qs ? `?${qs}` : ''}`
  const timeoutMs = options.timeoutMs ?? 5000

  const controller = new AbortController()
  const timeout = setTimeout(() => controller.abort(), timeoutMs)

  const events: SSEEvent[] = []
  try {
    const response = await fetch(url, {
      headers: { Accept: 'text/event-stream' },
      signal: controller.signal,
    })
    const reader = response.body?.getReader()
    if (!reader) return events

    const decoder = new TextDecoder()
    let buffer = ''

    while (true) {
      const { done, value } = await reader.read()
      if (done) break
      buffer += decoder.decode(value, { stream: true })

      // Parse SSE frames from the buffer.
      const parts = buffer.split('\n\n')
      buffer = parts.pop() ?? ''
      for (const part of parts) {
        let eventType = ''
        let data = ''
        for (const line of part.split('\n')) {
          if (line.startsWith('event: ')) eventType = line.slice(7)
          if (line.startsWith('data: ')) data = line.slice(6)
        }
        if (eventType && data) {
          try {
            events.push({ event: eventType, data: JSON.parse(data) as Record<string, unknown> })
          } catch { /* skip non-JSON */ }
        }
      }
    }
  } catch {
    // AbortError from timeout is expected.
  } finally {
    clearTimeout(timeout)
  }

  return events
}

/** Sends text to a worktree's terminal. */
export async function sendWorktreeInput(
  projectId: string,
  worktreeId: string,
  text: string,
  options?: { pressEnter?: boolean; agentType?: string },
): Promise<void> {
  const agentType = options?.agentType ?? 'claude-code'
  await apiFetch(
    `/api/v1/projects/${projectId}/${agentType}/worktrees/${worktreeId}/input`,
    {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        text,
        pressEnter: options?.pressEnter,
      }),
    },
  )
}

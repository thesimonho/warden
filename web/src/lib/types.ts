/** Network isolation level for a container. */
export type NetworkMode = 'full' | 'restricted' | 'none'

/** Whether Claude Code is actively running inside a container. */
export type ClaudeStatus = 'idle' | 'working' | 'unknown'

/** The kind of attention Claude Code needs from the user. */
export type NotificationType =
  | 'permission_prompt'
  | 'idle_prompt'
  | 'auth_success'
  | 'elicitation_dialog'

/** Terminal connection state of a worktree. */
export type WorktreeState = 'connected' | 'shell' | 'background' | 'disconnected'

/** Maps worktree state to a colored dot indicator, text color, and label. */
export const worktreeStateIndicator: Record<
  WorktreeState,
  { dotClass: string; textClass: string; label: string }
> = {
  connected: { dotClass: 'bg-success', textClass: 'text-success', label: 'Connected' },
  shell: { dotClass: 'bg-warning', textClass: 'text-warning', label: 'Claude exited' },
  background: {
    dotClass: 'bg-active animate-pulse',
    textClass: 'text-active',
    label: 'Active in background',
  },
  disconnected: {
    dotClass: 'bg-muted-foreground/40',
    textClass: 'text-muted-foreground',
    label: 'Disconnected',
  },
}

/** Returns true if the worktree has a connectable terminal (abduco alive). */
export function hasActiveTerminal(worktree: { state: WorktreeState }): boolean {
  return (
    worktree.state === 'connected' || worktree.state === 'shell' || worktree.state === 'background'
  )
}

/** Derives a human-readable label for the worktree state. */
export function deriveStateLabel(state: WorktreeState, exitCode?: number): string | undefined {
  if (state === 'shell') {
    return exitCode != null && exitCode !== 0 ? `Exited (${exitCode})` : 'Claude exited'
  }
  if (state === 'background') return 'Background'
  return undefined
}

/** Returns true if the worktree has Claude actively running. */
export function isConnectedWorktree(worktree: { state: WorktreeState }): boolean {
  return worktree.state === 'connected'
}

/** Returns true if the worktree has a live process (connected, shell, or background). */
export function isWorktreeAlive(worktree: { state: WorktreeState }): boolean {
  return worktree.state !== 'disconnected'
}

/** Represents a project managed by the dashboard. */
export interface Project {
  /** Stable 12-char hex hash identifying this project. */
  projectId: string
  /** Docker container ID (present when a container exists). */
  id: string
  name: string
  /** Absolute path on the host for the project directory. */
  hostPath: string
  type: string
  /** Docker image the container was built from. */
  image: string
  /** OS derived from OCI image labels, e.g. "ubuntu 24.04". */
  os: string
  /** Unix timestamp (seconds) when the container was created. */
  createdAt: number
  /** Host port mapped to SSH inside the container, if any. */
  sshPort: string
  /** Container state, e.g. "running", "exited", "paused". */
  state: string
  /** Docker status string, e.g. "Up 2 hours". */
  status: string
  /** Whether Claude Code is currently active in this container. */
  claudeStatus: ClaudeStatus
  /** True when any worktree requires user attention. */
  needsInput?: boolean
  /** Why Claude needs attention (permission_prompt, idle_prompt, elicitation_dialog). */
  notificationType?: NotificationType
  /** Whether a Docker container exists for this project. */
  hasContainer: boolean
  /** Number of worktrees with connected terminals. */
  activeWorktreeCount: number
  /** Aggregate cost across all worktrees in USD (from agent status provider). */
  totalCost: number
  /** True when cost is an estimate (subscription user) rather than actual API spend. */
  isEstimatedCost?: boolean
  /** Per-project cost limit in USD (0 = use global default). */
  costBudget: number
  /** Whether the container's /project is a git repository. */
  isGitRepo: boolean
  /** Whether terminals skip permission prompts. */
  skipPermissions: boolean
  /** Host directory mounted into the container (mount source). */
  mountedDir?: string
  /** Container-side workspace directory (mount destination). */
  workspaceDir?: string
  /** Network isolation level for the container. */
  networkMode: NetworkMode
  /** Domains accessible when networkMode is "restricted". */
  allowedDomains?: string[]
}

/** Host↔container path mapping for a project's workspace bind mount. */
export interface WorkspaceMount {
  /** Host directory mounted into the container (mount source). */
  mountedDir: string
  /** Container-side workspace directory (mount destination). */
  workspaceDir: string
}

/** Extracts the workspace mount from a project, if both paths are available. */
export function workspaceMount(project: Project): WorkspaceMount | undefined {
  return project.mountedDir && project.workspaceDir
    ? { mountedDir: project.mountedDir, workspaceDir: project.workspaceDir }
    : undefined
}

/** Represents a git worktree (or implicit project root) with its terminal state. */
export interface Worktree {
  /** Worktree identifier — directory name for git worktrees, "main" for project root. */
  id: string
  /** Container ID this worktree belongs to. */
  projectId: string
  /** Filesystem path inside the container. */
  path: string
  /** Git branch checked out in this worktree. */
  branch?: string
  /** Terminal connection state (connected, shell, disconnected). */
  state: WorktreeState
  /** Claude's exit code when in shell state. */
  exitCode?: number
  /** True when Claude is blocked waiting for user attention. */
  needsInput?: boolean
  /** Why Claude needs attention (permission_prompt, idle_prompt, elicitation_dialog). */
  notificationType?: NotificationType
}

/** Returns a display label for a worktree — project name for main, worktree ID otherwise. */
export function worktreeDisplayName(worktreeId: string, projectName: string): string {
  return worktreeId === 'main' ? projectName : worktreeId
}

// ---------------------------------------------------------------------------
// SSE event payloads (from GET /api/v1/events)
// ---------------------------------------------------------------------------

/** Payload for `worktree_state` SSE events. */
export interface WorktreeStateEvent {
  projectId: string
  containerName: string
  worktreeId: string
  needsInput: boolean
  notificationType?: NotificationType
  /** Whether a Claude session is currently running in this worktree. */
  sessionActive: boolean
  /** Terminal state derived from push events (overrides poll-based state when present). */
  state?: WorktreeState
  /** Claude's exit code (present when Claude has exited). */
  exitCode?: number
}

/**
 * Derives the next worktree state from an SSE event and the current state.
 *
 * Uses push-based terminal state when available in the event, otherwise falls
 * back to the session lifecycle heuristic (sessionActive → shell transition).
 *
 * @param event - The SSE worktree_state event.
 * @param currentState - The worktree's current state from the last poll/event.
 * @returns The derived worktree state.
 */
export function deriveWorktreeStateFromEvent(
  event: WorktreeStateEvent,
  currentState: WorktreeState,
): WorktreeState {
  if (event.state) return event.state
  if (!event.sessionActive && currentState === 'connected') return 'shell'
  return currentState
}

/** Payload for `project_state` SSE events. */
export interface ProjectStateEvent {
  projectId: string
  containerName: string
  totalCost: number
  messageCount: number
}

/** Payload for `worktree_list_changed` SSE events. */
export interface WorktreeListChangedEvent {
  projectId: string
  /** Container name whose worktree list changed. */
  containerName: string
}

/** Result of a worktree mutation (connect, disconnect, kill, remove). */
export interface WorktreeResult {
  worktreeId: string
  projectId: string
}

/** Result of a project mutation (add, remove, stop, restart). */
export interface ProjectResult {
  projectId: string
  name: string
  containerId?: string
}

/** Result of a container mutation (create, update, delete). */
export interface ContainerResult {
  containerId: string
  name: string
}

/** A bind mount from the host into the container. */
export interface Mount {
  /** Absolute path on the host. */
  hostPath: string
  /** Absolute path inside the container. */
  containerPath: string
  /** Whether the mount is read-only. */
  readOnly: boolean
}

/** Request body for creating a new project container. */
export interface CreateContainerRequest {
  name: string
  image: string
  projectPath: string
  envVars?: Record<string, string>
  /** Additional bind mounts from host into the container. */
  mounts?: Mount[]
  /** Whether terminals should skip permission prompts. */
  skipPermissions?: boolean
  /** Network isolation level for the container. */
  networkMode?: NetworkMode
  /** Domains accessible when networkMode is "restricted". */
  allowedDomains?: string[]
  /** Per-project cost limit in USD (0 = use global default). */
  costBudget?: number
}

/** Editable configuration of an existing container. */
export interface ContainerConfig {
  name: string
  image: string
  projectPath: string
  envVars?: Record<string, string>
  mounts?: Mount[]
  skipPermissions: boolean
  /** Network isolation level for the container. */
  networkMode: NetworkMode
  /** Domains accessible when networkMode is "restricted". */
  allowedDomains?: string[]
  /** Per-project cost limit in USD (0 = use global default). */
  costBudget: number
}

/** Information about a detected container runtime. */
export interface RuntimeInfo {
  /** Runtime identifier ("docker" or "podman"). */
  name: 'docker' | 'podman'
  /** Whether the runtime's API socket is reachable. */
  available: boolean
  /** Filesystem path to the runtime's API socket. */
  socketPath: string
  /** Runtime API version, if available. */
  version?: string
}

/** Audit log mode controls which events are written to the database. */
export type AuditLogMode = 'off' | 'standard' | 'detailed'

/** Server-side settings. */
export interface ServerSettings {
  /** Active container runtime. */
  runtime: 'docker' | 'podman'
  /** Audit log mode (off/standard/detailed). */
  auditLogMode: AuditLogMode
  /** Global default per-project cost budget in USD (0 = unlimited). */
  defaultProjectBudget: number

  /** Show a warning (toast + audit log) when a project exceeds its budget. */
  budgetActionWarn: boolean
  /** Kill all worktree processes when a project exceeds its budget. */
  budgetActionStopWorktrees: boolean
  /** Stop the project container when a project exceeds its budget. */
  budgetActionStopContainer: boolean
  /** Block starting/restarting a project that has exceeded its budget. */
  budgetActionPreventStart: boolean
}

/** Shared payload shape for budget enforcement SSE events. */
export interface BudgetEventPayload {
  projectId: string
  containerName: string
  totalCost: number
  budget: number
}

/** Payload for `budget_exceeded` SSE events. */
export type BudgetExceededEvent = BudgetEventPayload

/** Payload for `budget_container_stopped` SSE events. */
export interface BudgetContainerStoppedEvent extends BudgetEventPayload {
  containerId: string
}

/** Source layer that produced an audit log entry. */
export type AuditLogSource = 'agent' | 'backend' | 'frontend' | 'container'

/** Severity level for an audit log entry. */
export type AuditLogLevel = 'info' | 'warn' | 'error'

/** A single entry from the centralized audit log. */
export interface AuditLogEntry {
  /** ISO 8601 timestamp with milliseconds. */
  ts: string
  /** Origin layer. */
  source: AuditLogSource
  /** Severity level. */
  level: AuditLogLevel
  /** Snake_case event type identifier (e.g. "session_start", "container_create"). */
  event: string
  /** Stable project ID (empty for backend/frontend events without one). */
  projectId?: string
  /** Container name (for display). */
  containerName?: string
  /** Worktree ID (only for agent events). */
  worktree?: string
  /** Human-readable description. */
  msg?: string
  /** Raw event payload. */
  data?: Record<string, unknown>
  /** Structured key-value metadata. */
  attrs?: Record<string, unknown>
  /** Audit category (session, agent, prompt, config, system). Computed by the server. */
  category?: AuditCategory
}

/** A filesystem entry from the browser. */
export interface DirEntry {
  /** Entry name. */
  name: string
  /** Absolute path to the entry. */
  path: string
  /** Whether this entry is a directory. */
  isDir: boolean
}

// --- Diff ---

/** Per-file change statistics in a worktree diff. */
export interface DiffFileSummary {
  /** File path relative to the worktree root. */
  path: string
  /** Previous path for renamed files. */
  oldPath?: string
  /** Number of lines added. */
  additions: number
  /** Number of lines removed. */
  deletions: number
  /** Whether the file is binary. */
  isBinary: boolean
  /** Change type. */
  status: 'added' | 'modified' | 'deleted' | 'renamed'
}

/** Complete diff output for a worktree. */
export interface DiffResponse {
  /** Per-file change statistics. */
  files: DiffFileSummary[]
  /** Unified diff output from git. */
  rawDiff: string
  /** Sum of additions across all files. */
  totalAdditions: number
  /** Sum of deletions across all files. */
  totalDeletions: number
  /** True when the raw diff exceeded the size limit. */
  truncated: boolean
}

// --- Audit Log ---

/** Audit event category for filtering. */
export type AuditCategory =
  | 'session'
  | 'agent'
  | 'prompt'
  | 'config'
  | 'system'
  | 'budget'
  | 'debug'

/** Filters for the audit log API. */
export interface AuditFilters {
  projectId?: string
  worktree?: string
  category?: AuditCategory
  source?: string
  level?: string
  since?: string
  until?: string
  limit?: number
  offset?: number
}

/** Tool usage count for audit summary. */
export interface ToolCount {
  name: string
  count: number
}

/** Time range for audit summary. */
export interface TimeRange {
  earliest?: string
  latest?: string
}

/** Aggregate audit statistics. */
export interface AuditSummary {
  totalSessions: number
  totalToolUses: number
  totalPrompts: number
  totalCostUsd: number
  uniqueProjects: number
  uniqueWorktrees: number
  topTools: ToolCount[]
  timeRange: TimeRange
}

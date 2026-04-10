/**
 * CLI agent type running inside a project container.
 * String values must match the constants in agent/registry.go.
 */
export type AgentType = 'claude-code' | 'codex'

/** Default agent type for new projects. */
export const DEFAULT_AGENT_TYPE: AgentType = 'claude-code'

/** All supported agent types in display order. */
export const agentTypeOptions: AgentType[] = ['claude-code', 'codex']

/** Human-readable display labels for each agent type. */
export const agentTypeLabels: Record<AgentType, string> = {
  'claude-code': 'Claude Code',
  codex: 'OpenAI Codex',
}

/** Network isolation level for a container. */
export type NetworkMode = 'full' | 'restricted' | 'none'

/** Agent-specific overrides within a project template. */
export interface AgentTemplateOverride {
  allowedDomains?: string[]
}

/**
 * Project template loaded from a .warden.json file.
 *
 * Excluded fields (security): envVars (may contain secrets) and
 * accessItems (resolve to credentials).
 */
export interface ProjectTemplate {
  image?: string
  skipPermissions?: boolean
  networkMode?: NetworkMode
  costBudget?: number
  runtimes?: string[]
  /** Container ports to forward via reverse proxy. */
  forwardedPorts?: number[]
  agents?: Record<string, AgentTemplateOverride>
}

/** Whether the agent CLI is actively running inside a container. */
export type AgentStatus = 'idle' | 'working' | 'unknown'

/** The kind of attention Claude Code needs from the user. */
export type NotificationType =
  | 'permission_prompt'
  | 'idle_prompt'
  | 'auth_success'
  | 'elicitation_dialog'

/** Terminal connection state of a worktree. */
export type WorktreeState = 'connected' | 'shell' | 'background' | 'stopped'

/** Maps worktree state to a colored dot indicator, text color, and label. */
export const worktreeStateIndicator: Record<
  WorktreeState,
  { dotClass: string; textClass: string; label: string }
> = {
  connected: { dotClass: 'bg-success', textClass: 'text-success', label: 'Connected' },
  shell: { dotClass: 'bg-warning', textClass: 'text-warning', label: 'Agent exited' },
  background: {
    dotClass: 'bg-active animate-pulse',
    textClass: 'text-active',
    label: 'Active in background',
  },
  stopped: {
    dotClass: 'bg-muted-foreground/40',
    textClass: 'text-muted-foreground',
    label: 'Stopped',
  },
}

/** Returns true if the worktree has a connectable terminal (tmux session alive). */
export function hasActiveTerminal(worktree: { state: WorktreeState }): boolean {
  return (
    worktree.state === 'connected' || worktree.state === 'shell' || worktree.state === 'background'
  )
}

/** Derives a human-readable label for the worktree state. */
export function deriveStateLabel(state: WorktreeState, exitCode?: number): string | undefined {
  if (state === 'shell') {
    return exitCode != null && exitCode !== 0 ? `Exited (${exitCode})` : 'Agent exited'
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
  return worktree.state !== 'stopped'
}

/** Indicates whether a project uses a local host directory or remote git clone. */
export type ProjectSource = 'local' | 'remote'

/** Represents a project managed by the dashboard. */
export interface Project {
  /** Stable 12-char hex hash identifying this project. */
  projectId: string
  /** Docker container ID (present when a container exists). */
  id: string
  name: string
  /** Absolute path on the host for the project directory (local projects only). */
  hostPath: string
  /** Git repository URL to clone (remote projects only). */
  cloneURL?: string
  /** Whether this is a local or remote project. */
  source: ProjectSource
  /** True when a remote project's workspace is ephemeral (lost on container recreate). */
  temporary?: boolean
  /** The CLI agent type running in this project. */
  agentType: AgentType
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
  /** Whether the agent CLI is currently active in this container. */
  agentStatus: AgentStatus
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
  /** Pinned CLI version installed in this container. */
  agentVersion?: string
  /** Container ports exposed via the Warden reverse proxy. */
  forwardedPorts?: number[]
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
  /** Terminal connection state (connected, shell, background, stopped). */
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
  agentType?: AgentType
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
  agentType?: AgentType
  containerName: string
  totalCost: number
  messageCount: number
  needsInput: boolean
  notificationType?: NotificationType
}

/** Payload for `worktree_list_changed` SSE events. */
export interface WorktreeListChangedEvent {
  projectId: string
  agentType?: AgentType
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
  agentType: AgentType
  name: string
  containerId?: string
}

/** Result of POST /api/v1/projects (project + optional container). */
export interface AddProjectResponse {
  project: ProjectResult
  container?: ContainerResult
}

/** Result of a batch project operation. */
export interface BatchProjectResponse {
  results: Array<{
    projectId: string
    agentType: string
    success: boolean
    error?: string
  }>
}

/** Result of a container mutation (create, update, delete). */
export interface ContainerResult {
  containerId: string
  name: string
  /** True when the container was fully recreated (not just settings updated). */
  recreated?: boolean
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
  /** Git repository URL to clone (remote projects). */
  cloneURL?: string
  /** Whether the remote workspace is ephemeral. */
  temporary?: boolean
  /** CLI agent type (defaults to "claude-code" if omitted). */
  agentType?: AgentType
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
  /** Active access item IDs (e.g. ["git", "ssh"]). */
  enabledAccessItems?: string[]
  /** Active runtime IDs (e.g. ["node", "python", "go"]). */
  enabledRuntimes?: string[]
  /** Container ports to expose via the reverse proxy (1-65535). */
  forwardedPorts?: number[]
}

/** Editable configuration of an existing container. */
export interface ContainerConfig {
  name: string
  image: string
  projectPath: string
  /** Git repository URL (remote projects only). */
  cloneURL?: string
  /** Whether the remote workspace is ephemeral. */
  temporary?: boolean
  /** CLI agent type running in this project. */
  agentType: AgentType
  envVars?: Record<string, string>
  mounts?: Mount[]
  skipPermissions: boolean
  /** Network isolation level for the container. */
  networkMode: NetworkMode
  /** Domains accessible when networkMode is "restricted". */
  allowedDomains?: string[]
  /** Per-project cost limit in USD (0 = use global default). */
  costBudget: number
  /** Active access item IDs (e.g. ["git", "ssh"]). */
  enabledAccessItems?: string[]
  /** Active runtime IDs (e.g. ["node", "python", "go"]). */
  enabledRuntimes?: string[]
  /** Container ports exposed via the reverse proxy. */
  forwardedPorts?: number[]
}

// --- Runtimes ---

/** A language runtime available for installation in containers. */
export interface RuntimeDefault {
  /** Unique identifier (e.g. "node", "python", "go"). */
  id: string
  /** Human-readable name (e.g. "Node.js", "Python"). */
  label: string
  /** Brief description of what gets installed. */
  description: string
  /** Whether this runtime cannot be deselected (e.g. Node for MCP). */
  alwaysEnabled: boolean
  /** True when marker files were found in the project directory. */
  detected: boolean
  /** Network domains required for this runtime's package registry. */
  domains: string[]
  /** Environment variables set when this runtime is enabled. */
  envVars: Record<string, string>
}

// --- Access System ---

/** Source type for how to detect/read a credential on the host. */
export type AccessSourceType = 'env' | 'file' | 'socket' | 'named_pipe' | 'command'

/** Injection type for how to deliver a credential into the container. */
export type AccessInjectionType = 'env' | 'mount_file' | 'mount_socket'

/** A host source describing where a credential value lives. */
export interface AccessSource {
  /** The kind of host source. */
  type: AccessSourceType
  /** The env var name, file path, socket path, or command string. */
  value: string
}

/** An optional transformation applied between source resolution and injection. */
export interface AccessTransform {
  /** The transformation type (e.g. "strip_lines", "git_include"). */
  type: string
  /** Type-specific configuration parameters. */
  params?: Record<string, string>
}

/** Describes how a resolved credential is delivered into the container. */
export interface AccessInjection {
  /** The kind of container injection. */
  type: AccessInjectionType
  /** The env var name or container path for the injection target. */
  key: string
  /** Static override for the resolved value. */
  value?: string
  /** Whether the mount is read-only (for mount injections). */
  readOnly?: boolean
}

/** A single credential pairing host sources with container injections. */
export interface AccessCredential {
  /** Human-readable name for this credential. */
  label: string
  /** Sources tried in order; the first detected value is used. */
  sources: AccessSource[]
  /** Optional transformation applied to the resolved value. */
  transform?: AccessTransform
  /** Container-side delivery targets. */
  injections: AccessInjection[]
}

/** A named group of credentials for host-to-container passthrough. */
export interface AccessItem {
  /** Stable identifier. Built-in items use well-known IDs; user items get UUIDs. */
  id: string
  /** Human-readable display name. */
  label: string
  /** Explains what this access item provides. */
  description: string
  /** Delivery strategy (only "transport" for now). */
  method: string
  /** The individual credential entries in this group. */
  credentials: AccessCredential[]
  /** True for items that ship with Warden. */
  builtIn: boolean
}

/** Per-credential detection status on the current host. */
export interface AccessCredentialStatus {
  /** The credential's human-readable name. */
  label: string
  /** True when at least one source was detected. */
  available: boolean
  /** Which source was detected (empty when unavailable). */
  sourceMatched?: string
}

/** Detection result for an access item showing host availability. */
export interface AccessDetectionResult {
  /** The access item identifier. */
  id: string
  /** The access item display name. */
  label: string
  /** True when at least one credential was detected. */
  available: boolean
  /** Per-credential detection results. */
  credentials: AccessCredentialStatus[]
}

/** An access item enriched with host detection status. */
export interface AccessItemResponse extends AccessItem {
  /** Per-credential availability on the current host. */
  detection: AccessDetectionResult
}

/** A single resolved delivery into the container. */
export interface ResolvedInjection {
  /** The injection kind (env, mount_file, mount_socket). */
  type: AccessInjectionType
  /** The env var name or container path. */
  key: string
  /** The resolved content (env var value, host file path, or host socket path). */
  value: string
  /** Whether the mount is read-only. */
  readOnly?: boolean
}

/** Resolution output for a single credential. */
export interface ResolvedCredential {
  /** The credential's human-readable name. */
  label: string
  /** True when a source was detected and all injections were produced. */
  resolved: boolean
  /** Which source was matched (empty when unresolved). */
  sourceMatched?: string
  /** The resolved container-side deliveries. */
  injections?: ResolvedInjection[]
  /** Error message when resolution failed. */
  error?: string
}

/** Full resolution output for an access item. */
export interface ResolvedItem {
  /** The access item identifier. */
  id: string
  /** The access item display name. */
  label: string
  /** Per-credential resolution results. */
  credentials: ResolvedCredential[]
}

/** Information about the detected Docker runtime. */
export interface RuntimeInfo {
  /** Runtime identifier. */
  name: string
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
  runtime: string
  /** Server working directory (used for resolving relative paths). */
  workingDirectory: string
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
  /** Server build version (e.g. "v0.5.2", "dev"). */
  version: string
  /** Pinned Claude Code CLI version for containers. */
  claudeCodeVersion: string
  /** Pinned Codex CLI version for containers. */
  codexVersion: string
}

/** Shared payload shape for budget enforcement SSE events. */
export interface BudgetEventPayload {
  projectId: string
  agentType?: AgentType
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
  /** Database row identifier. Unique across all entries. */
  id: number
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
  /** Agent type associated with the event. */
  agentType?: AgentType
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

// --- Clipboard ---

/** Response from the clipboard upload endpoint. */
export interface ClipboardUploadResponse {
  path: string
}

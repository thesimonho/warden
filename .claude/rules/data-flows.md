# Data Flows and Lifecycle

## Worktree and terminal lifecycle

See `docs/terminology.md` for the full state machine (worktree states, terminal actions, Claude activity sub-states) and `docs/ux-flows.md` for the UX flows that use them.

The critical invariant: **WebSocket connections are disposable, abduco is not**. Disconnecting closes the WebSocket but leaves abduco alive so Claude keeps working in the background. Only an explicit "kill" destroys abduco.

## Terminal lifecycle

Browser connects via `GET /api/v1/projects/{id}/ws/{wid}` (WebSocket) → Go backend proxy (`internal/terminal/`) → `docker exec` with TTY mode attached to existing abduco session. Backend calls `create-terminal.sh` to initialize abduco for new worktrees.

## Container setup

### Symlink resolution

`engine/symlink_resolver.go` walks bind mount host paths to find symlinks whose targets escape the mounted tree (Nix Home Manager, GNU Stow, etc.) and adds extra bind mounts so they resolve inside the container. Called by `CreateContainer` before building bind mounts.

### Agent provider abstraction

The `StatusProvider` interface in `agent/` abstracts agent CLI differences (currently only Claude Code). Implement 3 methods (`Name()`, `ConfigFilePath()`, `ExtractStatus()`) and pass to `docker.NewClient()`. The provider is runtime-agnostic — it reads config via `docker exec` which works identically on both runtimes.

## Event system

### Attention tracking

Claude Code's hook events (Notification, PreToolUse, UserPromptSubmit) are pushed via `warden-event.sh` to a bind-mounted event directory (`WARDEN_EVENT_DIR`) → `eventbus/watcher.go` detects files and parses events → `eventbus/store.go` tracks attention state → SSE broadcasts to frontend. The watcher watches the directory using fsnotify (fast path) + polling every 2s (reliable fallback). Filesystem permissions handle access control (no bearer token needed). `UserPromptSubmit` fires two events: `attention_clear` (real-time state) and `user_prompt` (logged with prompt text, truncated to 500 chars).

### Hook data enrichment

`warden-event.sh` forwards additional data from Claude Code hooks:

- `session_start` includes `sessionId`, `model`, `source`
- `session_end` includes `reason`
- `pre_tool_use` sends both a `tool_use` audit event (with `toolName`, `toolInput` truncated to 1000 chars) and an attention state event
- `post_tool_use_failure` sends a `tool_use_failure` event (with `toolName`)
- `notification` maps to `attention` event type
- `user_prompt` includes `prompt` text

## Cost tracking

### Agent status and cost capture

Claude Code writes metrics to `~/.claude.json` inside the container, keyed by workspace path (unique per container via `WARDEN_WORKSPACE_DIR`). Cost is stored in the `session_costs` DB table keyed by `(project_id, session_id)` with `created_at` (first seen) and `updated_at` (last upsert) timestamps — cost per session is monotonically non-decreasing so upsert is always safe. Project total = `SUM(cost)` across all sessions for a `project_id`. Time-filtered cost (used by audit summary) queries sessions whose time span (`created_at`..`updated_at`) overlaps the requested range.

For running containers with no DB data yet, `ReadAgentCostAndBillingType` reads via docker exec and persists via `PersistSessionCost`. Cost is captured at multiple points: stop events (via hooks with `sessionId`), session start/end, after Claude exits (`warden-capture-cost.sh`), and before container stop (`readAndPersistAgentCost`). All paths funnel through `PersistSessionCost(projectID, containerName, sessionID, cost, isEstimated)` which handles both DB writes (keyed by `projectID`) and budget enforcement. The in-memory event store still broadcasts cost updates via SSE for real-time frontend updates but is not used as a read source. The `IsEstimatedCost` flag is derived from `oauthAccount.billingType` in `.claude.json`.

### Cost budgets

All cost writes go through `service.PersistSessionCost(projectID, containerName, sessionID, cost, isEstimated)` — the single gateway that persists cost to DB keyed by `projectID` and triggers budget enforcement (analogous to `db.AuditWriter.Write` for audit events).

Three entry points all funnel through it:

1. Event bus stop callback on every stop event (`StopCallbackFunc` signature: `(projectID, containerName, sessionID string, cost float64, isEstimated bool)`)
2. `readAndPersistAgentCost` docker exec fallback per session
3. Explicit calls with empty data for enforcement-only checks

Four user-configurable enforcement actions (stored in `settings` table):

| Action                     | Behavior                                                                                                            |
| -------------------------- | ------------------------------------------------------------------------------------------------------------------- |
| **Show warning** (default) | Broadcasts `budget_exceeded` SSE event and logs to audit log                                                        |
| **Stop worktrees**         | Kills all worktree processes                                                                                        |
| **Stop container**         | Stops the Docker container and broadcasts `budget_container_stopped` SSE event so frontends can redirect users away |
| **Prevent restart**        | Blocks `RestartProject` with 403                                                                                    |

`IsOverBudget(projectID)` checks cost + preventStart flag for blocking restarts. Audit events (`budget_exceeded`, `budget_worktrees_stopped`, `budget_container_stopped`, `budget_enforcement_failed`) are written via `db.AuditWriter`. Budget SSE payloads share the `BudgetEventPayload` struct (`projectId`, `containerName`, `totalCost`, `budget`); the `budget_container_stopped` variant extends it with `containerId` via `BudgetContainerStoppedPayload`. Settings are exposed in both the web settings dialog and TUI settings view.

## Audit system

### API endpoints

| Method   | Path                                    | Description                                                                                 |
| -------- | --------------------------------------- | ------------------------------------------------------------------------------------------- |
| `GET`    | `/api/v1/audit`                         | Returns audit events filtered by category, project, worktree, source, level, and time range |
| `GET`    | `/api/v1/audit/summary`                 | Returns aggregate statistics (session count, tool count, prompt count, cost)                |
| `POST`   | `/api/v1/audit`                         | Adds custom audit events                                                                    |
| `DELETE` | `/api/v1/audit`                         | Clears audit events (scoped with query params)                                              |
| `GET`    | `/api/v1/audit/export?format=csv\|json` | Compliance-ready downloads                                                                  |

Supports `source` (agent/backend/frontend/container) and `level` (info/warning/error) query params. The audit system reuses the existing event log infrastructure (same `events` SQLite table) but with audit-specific query filters and presentation.

### Mode filtering

`auditLogMode` setting (off/standard/detailed) controls what events are written to the audit DB. All audit writes go through a single `db.AuditWriter` interface that enforces mode filtering via a `standardEvents` allowlist before writing to the `events` SQLite table.

| Mode         | What gets written                                                            |
| ------------ | ---------------------------------------------------------------------------- |
| **Off**      | No events written                                                            |
| **Standard** | Session lifecycle, budget enforcement, system events, frontend-posted events |
| **Detailed** | Standard + agent events, prompt, config, debug                               |

The `AuditWriter` is the only path for audit log writes; direct `db.Store.Write()` calls for audit events are prohibited. Settings endpoint returns `auditLogMode` string.

### Categories

Seven categories partition all audit events:

| Category    | Event types                                                                                                    |
| ----------- | -------------------------------------------------------------------------------------------------------------- |
| **session** | session_start/end, terminal lifecycle, worktree lifecycle                                                      |
| **agent**   | tool_use, tool_use_failure, permission_request, subagent events                                                |
| **prompt**  | user_prompt                                                                                                    |
| **config**  | config_change, instructions_loaded                                                                             |
| **budget**  | budget_exceeded, budget_worktrees_stopped, budget_container_stopped, budget_enforcement_failed                 |
| **system**  | process_killed, restart_blocked, frontend events, project_removed, container_deleted, cost_reset, audit_purged |
| **debug**   | slog-captured backend warnings/errors                                                                          |

Mode-filtered writes use the `standardEvents` allowlist to exclude agent/prompt/config/debug events when mode is Standard.

## Persistence

### Project data

Project config (project_id, name, host_path, image, mounts, env vars, network mode, skipPermissions, costBudget, container_id, container_name) is stored in the `projects` SQLite table, keyed by `project_id` (deterministic hash of host path). Per-session costs are in the `session_costs` table (project_id, session_id, cost, is_estimated, updated_at), keyed by the same `project_id` for cost aggregation and enforcement. Settings (runtime, auditLogMode, disconnectKey, defaultProjectBudget, budgetAction{Warn,StopWorktrees,StopContainer,PreventStart}) are in the `settings` table.

The service layer reads from DB and overlays onto engine results. Docker labels are no longer used for discovery (warden.discover removed). When a project is deleted: if audit logging is enabled (standard/detailed), cost data and events are preserved for audit history; if audit logging is off, all associated costs and events are cleaned up.

### Project management dialog

The management dialog exposes four independent destructive actions as unchecked checkboxes (nothing checked by default):

1. **Remove from Warden** — untrack project from DB, emits `project_removed`
2. **Delete container** — stop + remove Docker container, emits `container_deleted`
3. **Reset cost history** — `DELETE /api/v1/projects/{projectId}/costs`, emits `cost_reset`
4. **Purge audit history** — `DELETE /api/v1/projects/{projectId}/audit`, emits `audit_purged` write-ahead then deletes all events

Audit purge requires type-to-confirm. Dialog keeps open on partial failure. Cost reset and audit purge run in parallel when both selected. Container deletion runs first, project removal runs last.

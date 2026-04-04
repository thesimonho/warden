---
paths:
  - "db/**/*"
  - "service/**/*"
  - "internal/server/**/*"
  - "api/**/*"
  - "eventbus/**/*"
  - "container/scripts/**/*"
  - "web/src/**/audit*"
---

# Audit System

## API endpoints

| Method   | Path                                    | Description                                                                                 |
| -------- | --------------------------------------- | ------------------------------------------------------------------------------------------- |
| `GET`    | `/api/v1/audit`                         | Returns audit events filtered by category, project, worktree, source, level, and time range |
| `GET`    | `/api/v1/audit/summary`                 | Returns aggregate statistics (session count, tool count, prompt count, cost)                |
| `POST`   | `/api/v1/audit`                         | Adds custom audit events                                                                    |
| `DELETE` | `/api/v1/audit`                         | Clears audit events (scoped with query params)                                              |
| `GET`    | `/api/v1/audit/export?format=csv\|json` | Compliance-ready downloads                                                                  |

Supports `source` (agent/backend/frontend/container) and `level` (info/warning/error) query params.

## Mode filtering

`auditLogMode` setting (off/standard/detailed) controls what events are written to the audit DB. All audit writes MUST go through a single `db.AuditWriter` interface that enforces mode filtering via a `standardEvents` allowlist before writing to the `events` SQLite table.

| Mode         | What gets written                                                            |
| ------------ | ---------------------------------------------------------------------------- |
| **Off**      | No events written                                                            |
| **Standard** | Session lifecycle, budget enforcement, system events, frontend-posted events |
| **Detailed** | Standard + agent events, prompt, config, debug                               |

The `AuditWriter` is the only path for audit log writes; direct `db.Store.Write()` calls for audit events are prohibited. Settings endpoint returns `auditLogMode` string.

## Categories

Seven categories partition all audit events:

| Category    | Event types                                                                                                                                                                                                         |
| ----------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **session** | session_start/end, terminal lifecycle, container_heartbeat_stale, worktree lifecycle                                                                                                                                |
| **agent**   | tool_use, tool_use_failure, permission_request, subagent events                                                                                                                                                     |
| **prompt**  | user_prompt                                                                                                                                                                                                         |
| **config**  | config_change, instructions_loaded                                                                                                                                                                                  |
| **budget**  | budget_exceeded, budget_worktrees_stopped, budget_container_stopped, budget_enforcement_failed, cost_reset, cost_snapshot                                                                                           |
| **system**  | process_killed, restart_blocked, frontend events, project_removed, container_created, container_deleted, container_updated, audit_purged, access_item_created/updated/deleted/reset, api_metrics, runtime_installed |
| **debug**   | slog-captured backend warnings/errors                                                                                                                                                                               |

Mode-filtered writes use the `standardEvents` allowlist to exclude agent/prompt/config/debug events when mode is Standard.

## Persistence

Project config (project_id, agent_type, name, host_path, image, mounts, env vars, network mode, skipPermissions, costBudget, enabled_access_items, container_id, container_name) is stored in the `projects` SQLite table, keyed by `(project_id, agent_type)` compound primary key. Per-session costs are in the `session_costs` table (project_id, agent_type, session_id, cost, is_estimated, created_at, updated_at), keyed by `(project_id, agent_type, session_id)` for cost aggregation and enforcement. Audit events are in the `events` table (project_id, agent_type, category, source, level, and other audit data), keyed by `(project_id, agent_type)` for audit scoping. Access items are in the `access_items` table (id, label, description, method, credentials JSON) for user-defined items; built-in items are provided by the access/ library. Settings (runtime, auditLogMode, disconnectKey, defaultProjectBudget, budgetAction{Warn,StopWorktrees,StopContainer,PreventStart}) are in the `settings` table.

The service layer reads from DB and overlays onto engine results. When a project is deleted: if audit logging is enabled (standard/detailed), cost data and events are preserved for audit history; if audit logging is off, all associated costs and events are cleaned up.

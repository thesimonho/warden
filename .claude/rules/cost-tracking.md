---
paths:
  - "service/**/*"
  - "db/**/*"
  - "eventbus/**/*"
  - "internal/server/**/*"
  - "api/**/*"
  - "web/src/**/cost*"
  - "web/src/**/budget*"
  - "web/src/**/settings*"
  - "container/scripts/**/*"
---

# Cost Tracking

## Agent status and cost capture

Claude Code writes metrics to `~/.claude.json` inside the container, keyed by workspace path (unique per container via `WARDEN_WORKSPACE_DIR`). Cost is stored in the `session_costs` DB table keyed by `(project_id, session_id)` with `created_at` (first seen) and `updated_at` (last upsert) timestamps — cost per session is monotonically non-decreasing so upsert is always safe. Project total = `SUM(cost)` across all sessions for a `project_id`. Time-filtered cost (used by audit summary) queries sessions whose time span (`created_at`..`updated_at`) overlaps the requested range.

For running containers with no DB data yet, `ReadAgentCostAndBillingType` reads via docker exec and persists via `PersistSessionCost`. Cost is captured at multiple points: stop events (via hooks with `sessionId`), session start/end, after Claude exits (`warden-capture-cost.sh`), and before container stop (`readAndPersistAgentCost`). All paths funnel through `PersistSessionCost(projectID, containerName, sessionID, cost, isEstimated)` which handles both DB writes (keyed by `projectID`) and budget enforcement. The in-memory event store still broadcasts cost updates via SSE for real-time frontend updates but is not used as a read source. The `IsEstimatedCost` flag is derived from `oauthAccount.billingType` in `.claude.json`.

## Cost budgets

All cost writes MUST go through `service.PersistSessionCost(projectID, containerName, sessionID, cost, isEstimated)` — the single gateway that persists cost to DB keyed by `projectID` and triggers budget enforcement (analogous to `db.AuditWriter.Write` for audit events).

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

# Cost Tracking & Budgets

Warden automatically tracks agent spending per project and can enforce budget limits -- warning you, stopping worktrees, or shutting down containers when costs exceed thresholds.

This page covers cost and budget concepts and HTTP API patterns. See [api/projects.md](./api/projects.md) and [api/settings.md](./api/settings.md) for full request/response field definitions.

## How cost tracking works

Costs are captured automatically per project from **JSONL session files** that each agent writes during operation. Cost data is persisted to the database keyed by `(projectID, agentType, sessionID)` and survives container restarts and recreation -- costs are tied to the project, not the container.

Cost is captured at multiple points during the agent lifecycle:

- On cost update events (from JSONL token counts during operation)
- On session start and end
- After the agent exits (via a capture script inside the container)
- Before container stop

All cost writes funnel through a single internal gateway (`PersistSessionCost`) which handles both database persistence and budget enforcement. This gateway is never bypassed -- every cost change triggers a budget check.

The total cost for a project is the sum of all session costs for its `(projectID, agentType)` pair. Cost data is included in the project list response under `totalCost`.

## Cost models

Cost accuracy depends on the agent type and billing method:

| Model | When it applies | Accuracy |
|-------|----------------|----------|
| **Actual cost** | Claude Code with API key (`ANTHROPIC_API_KEY`) | Exact -- read from Claude's cost data |
| **Subscription estimate** | Claude Code with Pro/Max subscription | Estimated from token usage and model pricing |
| **Estimated** | Codex (all billing types) | Computed from token counts using pricing tables |

Codex costs are always estimated because the Codex CLI does not report actual spend. Warden computes cost from token usage and model-specific pricing tables. These estimates are directionally accurate but may not match your actual OpenAI bill exactly.

The `isEstimatedCost` field in cost data indicates whether the cost is a precise measurement or an estimate.

## Per-project budget

Set a cost limit on individual projects via the `costBudget` field when creating or updating a container. When the project's total cost crosses this threshold, enforcement actions fire.

Set `costBudget` to `0` (or omit it) to fall back to the global default budget.

### Setting a budget on container create

```bash
curl -s -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d '{
    "costBudget": 25.00
  }'
```

### Updating a budget on an existing container

```bash
curl -s -X PUT http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d '{
    "costBudget": 50.00
  }'
```

## Global default budget

Set a default budget that applies to all projects without a per-project override. Configure this via the settings API:

```bash
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{
    "defaultProjectBudget": 10.00
  }'
```

Set to `0` to disable budget enforcement globally.

## Enforcement actions

When a project exceeds its budget, Warden takes one or more configured actions. These are global settings that apply to all projects.

| Action | Setting key | Default | What it does |
|--------|------------|---------|-------------|
| **Warn** | `budgetActionWarn` | `true` | Broadcasts a `budget_exceeded` SSE event and writes an audit entry |
| **Stop worktrees** | `budgetActionStopWorktrees` | `false` | Kills all worktree processes in the container. The agent stops but the container stays running. |
| **Stop container** | `budgetActionStopContainer` | `false` | Stops the entire container. Broadcasts a `budget_container_stopped` SSE event so frontends can react. |
| **Prevent restart** | `budgetActionPreventStart` | `false` | Blocks container restart attempts. Returns HTTP 403 with error code `BUDGET_EXCEEDED` until the budget is increased or costs are reset. |

Actions are **cumulative** -- you can enable multiple simultaneously. For example, enabling both warn and stop container will show the warning and then shut down the container.

### Configuring enforcement actions

```bash
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{
    "budgetActionWarn": true,
    "budgetActionStopWorktrees": true,
    "budgetActionStopContainer": false,
    "budgetActionPreventStart": false
  }'
```

## Budget SSE events

Two SSE event types are relevant for real-time budget monitoring:

| Event | When it fires | Payload fields |
|-------|--------------|----------------|
| `budget_exceeded` | Project cost crosses the budget threshold | `projectId`, `containerName`, `totalCost`, `budget` |
| `budget_container_stopped` | Container stopped due to budget enforcement | `projectId`, `containerName`, `totalCost`, `budget`, `containerId` |

Listen for these on the SSE stream to update your UI in real time or trigger external alerts.

## Resetting costs

Clear all cost records for a project. This resets the budget counter to zero so the project can continue operating within its original budget.

```bash
curl -s -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/costs
```

Returns `204 No Content` on success.

This operation:

- Removes all session cost records for the project
- Resets the effective budget usage to zero
- Writes a `cost_reset` audit event (if audit logging is enabled)
- Broadcasts a `project_state` SSE event with the updated cost

## Reading current costs

Project costs are included in the project list response. There is no separate cost-only endpoint -- query the project to see its current spend:

```bash
curl -s http://localhost:8090/api/v1/projects | jq '.[0].totalCost'
```

For aggregate cost across all projects (from the audit system):

```bash
curl -s http://localhost:8090/api/v1/audit/summary | jq '.totalCostUsd'
```

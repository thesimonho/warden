---
title: Cost & Budget
description: Track per-project spending and enforce cost limits.
---

Warden tracks agent spending per project and can enforce budget limits automatically — warning you, stopping worktrees, or shutting down containers when costs exceed thresholds.

## How Cost Tracking Works

Warden automatically captures agent usage costs per project. Costs are recorded when the agent exits, when a container stops, and periodically during long-running worktrees. Costs survive container restarts and recreation — they're tied to the project, not the container.

### Cost Models

Cost accuracy depends on the agent type and billing method:

| Model | When it applies | Accuracy |
|-------|----------------|----------|
| **Actual cost** | Claude Code with API key (`ANTHROPIC_API_KEY`) | Exact — read from Claude's cost data |
| **Subscription cost** | Claude Code with Pro/Max subscription | Estimated from token usage |
| **Estimated cost** | Codex (all billing types) | Computed from token counts using pricing tables |

Codex costs are always estimated because the Codex CLI does not report actual spend. Warden computes cost from token usage and model-specific pricing tables. These estimates are directionally accurate but may not match your actual OpenAI bill exactly.

## Setting Budgets

### Per-Project Budget

Set a cost limit on individual projects via the container configuration. When the project's total cost crosses this threshold, enforcement actions fire.

Set to 0 (or leave blank) to fall back to the global default.

### Global Default Budget

Set a default budget that applies to all projects without a per-project override. Configure this in **Settings**. Set to 0 to disable budget enforcement globally.

### Resetting Costs

You can reset a project's accumulated cost at any time. This clears all cost records for that project and writes an audit event. The budget resets to the full amount.

## Enforcement Actions

When a project exceeds its budget, Warden can take one or more actions. Configure these in **Settings** — they apply globally to all projects.

| Action | Default | What it does |
|--------|---------|-------------|
| **Warn** | On | Shows a warning notification and writes an audit entry. |
| **Stop Worktrees** | Off | Kills all worktree processes in the project's container. Claude stops working but the container stays running. |
| **Stop Container** | Off | Stops the entire container. All worktrees are terminated. |
| **Prevent Restart** | Off | Blocks container restart attempts for over-budget projects. Returns an error until the budget is increased or costs are reset. |

Actions are cumulative — you can enable multiple actions simultaneously. For example, enabling both **Warn** and **Stop Container** will show the warning and then shut down the container.

:::tip
Start with just **Warn** enabled to get visibility into spending patterns. Add stricter enforcement once you've calibrated your budgets.
:::

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `DELETE` | `/api/v1/projects/{projectId}/costs` | Reset project costs |
| `GET` | `/api/v1/settings` | Get settings (includes budget config) |
| `PUT` | `/api/v1/settings` | Update settings (budget, enforcement actions) |

Budget configuration is part of the settings object:

```json
{
  "defaultCostBudget": 10.00,
  "budgetActionWarn": true,
  "budgetActionStopWorktrees": false,
  "budgetActionStopContainer": false,
  "budgetActionPreventStart": false
}
```

Per-project budgets are set via the container configuration when creating or updating a container (`costBudget` field in the request body).

### Go Client

```go
c := client.New("http://localhost:8090")

// Reset a project's accumulated costs
c.ResetProjectCosts(ctx, projectID)

// Configure budget enforcement
c.UpdateSettings(ctx, api.UpdateSettingsRequest{
    DefaultCostBudget:        floatPtr(10.00),
    BudgetActionWarn:         boolPtr(true),
    BudgetActionStopWorktrees: boolPtr(false),
    BudgetActionStopContainer: boolPtr(true),
    BudgetActionPreventStart:  boolPtr(false),
})
```

### Go Library

```go
w, _ := warden.New(warden.Options{})

// Reset costs
w.Service.ResetProjectCosts(projectID)
```

Budget enforcement happens automatically when Warden captures cost data — no manual triggering needed.

See the [Go Packages](/warden/reference/go/) reference for full API documentation.

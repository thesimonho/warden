---
title: Cost & Budget
description: Track per-project spending and enforce cost limits.
---

Warden tracks Claude Code spending per project and can enforce budget limits automatically — warning you, stopping worktrees, or shutting down containers when costs exceed thresholds.

## How Cost Tracking Works

Warden automatically captures Claude Code usage costs per project. Costs are recorded when Claude exits, when a container stops, and periodically during long-running worktrees. Costs survive container restarts and recreation — they're tied to the project, not the container.

### Estimated vs Actual

Cost accuracy depends on your Claude Code billing type:

- **API key users** — costs are actual spend
- **Subscription users** — costs are estimated based on token usage

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
app, _ := warden.New(warden.Options{})

// Reset costs
app.Service.ResetProjectCosts(projectID)
```

Budget enforcement happens automatically when Warden captures cost data — no manual triggering needed.

See the [Go Packages](/warden/reference/go/) reference for full API documentation.

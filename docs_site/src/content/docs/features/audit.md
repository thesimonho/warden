---
title: Audit Logging
description: Event logging for monitoring, compliance, and debugging.
---

Warden provides unified event logging across all your projects. Use it for real-time monitoring, post-incident review, or compliance reporting.

Audit logging is **off by default**. Enable it from Settings.

<a href="/warden/audit.webp" target="_blank">![](/warden/audit.webp)</a>

## Audit Modes

| Mode | What gets logged | Use case |
|------|-----------------|----------|
| **Off** | Nothing. | Default — no overhead. |
| **Standard** | Terminal lifecycle, worktree events, budget enforcement, system operations. | Day-to-day monitoring and cost oversight. |
| **Detailed** | Everything in Standard, plus tool use, permission requests, subagent activity, user prompts, config changes, and debug events. | Full audit trail for compliance or debugging. |

:::note
Switching modes takes effect immediately. Events that occurred before enabling audit logging are not captured retroactively.
:::

## Event Categories

Events are organized into seven categories:

| Category | What it captures | Minimum mode |
|----------|-----------------|--------------|
| **Session** | Container and terminal lifecycle — starts, stops, connects, disconnects | Standard |
| **Agent** | Claude's tool use, permission requests, subagent activity | Detailed |
| **Prompt** | User prompts sent to Claude | Detailed |
| **Config** | Settings changes, instruction file loading | Detailed |
| **Budget** | Budget exceeded warnings, enforcement actions, cost resets | Standard |
| **System** | Process kills, project/container deletion, audit purges | Standard |
| **Debug** | Backend warnings and errors | Detailed |

## Querying the Audit Log

Audit events can be filtered across several dimensions:

- **Category** — filter by one or more of the categories listed above
- **Project** — filter to a specific project
- **Source** — where the event originated (agent, backend, frontend, container)
- **Level** — info, warn, or error
- **Time range** — start and end timestamps
- **Worktree** — filter to a specific worktree

## Summary & Export

Warden aggregates audit statistics — total tool uses, prompts, cost across all projects, unique worktrees, and top tools used. This data can be exported as **CSV** or **JSON** for compliance review. Exports respect the current filter state.

## Data Lifecycle

### When a project is deleted

- **Audit logging on** (Standard or Detailed): cost data and audit events are preserved. The audit trail remains intact for historical review.
- **Audit logging off**: all associated costs and events are cleaned up with the project.

### Purging audit data

You can delete audit events scoped by:

- **Project** — remove all events for a specific project
- **Time range** — remove events before/after a timestamp
- **Worktree** — remove events for a specific worktree

Purge operations are themselves logged as audit events (category: System).

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/audit` | Query events with filters |
| `GET` | `/api/v1/audit/summary` | Aggregate statistics |
| `GET` | `/api/v1/audit/export?format=csv\|json` | Export filtered events |
| `GET` | `/api/v1/audit/projects` | List distinct project IDs with events |
| `POST` | `/api/v1/audit` | Write a custom event (for integrations) |
| `DELETE` | `/api/v1/audit` | Delete events matching filters |
| `DELETE` | `/api/v1/projects/{projectId}/audit` | Purge all audit data for a project |

Query parameters for `GET /api/v1/audit`:

| Parameter | Type | Description |
|-----------|------|-------------|
| `category` | string | Filter by category (comma-separated) |
| `source` | string | Filter by source |
| `level` | string | Filter by level |
| `projectId` | string | Filter by project |
| `worktree` | string | Filter by worktree |
| `since` | string | ISO 8601 start time |
| `until` | string | ISO 8601 end time |
| `limit` | int | Max events to return (default: 10,000) |
| `offset` | int | Pagination offset |

### Go Client

```go
c := client.New("http://localhost:8090")

// Query audit events
events, _ := c.GetAuditLog(ctx, api.AuditFilters{
    Category:  "agent",
    ProjectID: projectID,
    Since:     time.Now().Add(-24 * time.Hour),
})

// Get summary statistics
summary, _ := c.GetAuditSummary(ctx, api.AuditFilters{})

// Post a custom event (for integrations)
c.PostAuditEvent(ctx, api.PostAuditEventRequest{
    Event:   "deployment_started",
    Message: "Deploying v2.1.0 to staging",
    Level:   "info",
})

// Delete old events
c.DeleteAuditEvents(ctx, api.AuditFilters{
    Until: time.Now().Add(-30 * 24 * time.Hour),
})
```

### Go Library

```go
app, _ := warden.New(warden.Options{})

// Query events
events, _ := app.Service.GetAuditLog(api.AuditFilters{
    Category: "session",
    Limit:    100,
})

// Export as CSV
var buf bytes.Buffer
app.Service.WriteAuditCSV(&buf, api.AuditFilters{})
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.

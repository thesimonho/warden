# Audit Logging

Warden provides unified event logging across all projects. Use it for monitoring agent activity, post-incident review, compliance reporting, or building dashboards on top of agent behavior.

Audit logging is **off by default**. Enable it by updating the `auditLogMode` setting via the settings API. See [settings.md](./settings.md) for details.

This page covers audit concepts and HTTP API patterns. See [api/audit.md](./api/audit.md) for full request/response field definitions.

## Audit modes

The `auditLogMode` setting controls what gets recorded. Changing the mode takes effect immediately -- running containers are synced automatically. Events that occurred before enabling audit are not captured retroactively.

| Mode | What gets logged | Use case |
|------|-----------------|----------|
| `off` | Nothing | Default. Zero overhead. |
| `standard` | Terminal lifecycle, worktree events, budget enforcement, system operations | Day-to-day monitoring and cost oversight. |
| `detailed` | Everything in standard, plus tool use, permission requests, subagent activity, user prompts, config changes, and debug events | Full audit trail for compliance or debugging. |

## Event categories

Events are organized into seven categories. Each category requires a minimum audit mode to be captured:

| Category | What it captures | Minimum mode |
|----------|-----------------|--------------|
| `session` | Container and terminal lifecycle -- starts, stops, connects, disconnects, heartbeat staleness | `standard` |
| `agent` | Tool use, tool failures, permission requests, subagent activity | `detailed` |
| `prompt` | User prompts sent to the agent | `detailed` |
| `config` | Settings changes, instruction file loading | `detailed` |
| `budget` | Budget exceeded warnings, enforcement actions (worktrees stopped, container stopped), cost resets | `standard` |
| `system` | Process kills, restart blocks, project/container deletion, audit purges, access item changes, container creation/update, runtime installs | `standard` |
| `debug` | Backend warnings and errors captured from the server's structured logger | `detailed` |

## Data sources

Audit events come from two sources:

- **JSONL session files** (primary) -- each agent writes session JSONL files that Warden tails in real time. This provides session lifecycle, tool use, cost, and prompt events for both Claude Code and Codex.
- **Claude Code hooks** (supplementary) -- provides attention and notification state (needs permission, needs input, etc.). Codex does not support hooks, so attention-related audit events are not available for Codex projects.

## Querying events

Retrieve audit events with optional filters. All filter parameters are optional -- omit them to get all events.

```bash
# Get all events (up to default limit of 10000)
curl -s http://localhost:8090/api/v1/audit | jq

# Filter by category and project
curl -s 'http://localhost:8090/api/v1/audit?category=agent&projectId=a1b2c3d4e5f6' | jq

# Filter by time range (RFC 3339 timestamps)
curl -s 'http://localhost:8090/api/v1/audit?since=2025-01-01T00:00:00Z&until=2025-01-31T23:59:59Z' | jq

# Filter by source and level
curl -s 'http://localhost:8090/api/v1/audit?source=agent&level=error' | jq

# Paginate results
curl -s 'http://localhost:8090/api/v1/audit?limit=50&offset=100' | jq
```

### Available filters

| Parameter | Type | Description |
|-----------|------|-------------|
| `projectId` | string | Filter to a specific project |
| `worktree` | string | Filter to a specific worktree ID |
| `category` | string | One of: `session`, `agent`, `prompt`, `config`, `budget`, `system`, `debug` |
| `source` | string | Where the event originated: `agent`, `backend`, `frontend`, `container` |
| `level` | string | Severity: `info`, `warn`, `error` |
| `since` | string | Events after this timestamp (RFC 3339) |
| `until` | string | Events before this timestamp (RFC 3339) |
| `limit` | int | Maximum entries to return (default 10000) |
| `offset` | int | Number of entries to skip for pagination |

## Summary

Get aggregate statistics across all audit events, optionally filtered by project, worktree, or time range:

```bash
# Full summary
curl -s http://localhost:8090/api/v1/audit/summary | jq

# Summary for a specific project in the last 7 days
curl -s 'http://localhost:8090/api/v1/audit/summary?projectId=a1b2c3d4e5f6&since=2025-06-01T00:00:00Z' | jq
```

The response includes:

- `totalSessions` -- number of session start events
- `totalToolUses` -- number of tool use events
- `totalPrompts` -- number of user prompt events
- `totalCostUsd` -- aggregate cost across matching projects
- `uniqueProjects` / `uniqueWorktrees` -- distinct counts
- `topTools` -- most frequently used tools with counts
- `timeRange` -- earliest and latest event timestamps

## Export

Download audit events as CSV or JSONL for compliance review. Exports respect the same filters as the query endpoint.

```bash
# Export as CSV
curl -s 'http://localhost:8090/api/v1/audit/export?format=csv' -o audit.csv

# Export as JSONL (default format)
curl -s 'http://localhost:8090/api/v1/audit/export?format=json' -o audit.jsonl

# Export filtered events
curl -s 'http://localhost:8090/api/v1/audit/export?format=csv&projectId=a1b2c3d4e5f6&since=2025-01-01T00:00:00Z' -o audit.csv
```

The response includes a `Content-Disposition` header with a timestamped filename (e.g. `warden-audit-2025-06-15T143022.csv`).

## Writing custom events

Post custom audit events to record integration-specific activity alongside agent events. Useful for logging deployment triggers, CI notifications, or external tool invocations.

```bash
curl -s -X POST http://localhost:8090/api/v1/audit \
  -H "Content-Type: application/json" \
  -d '{
    "event": "deployment_triggered",
    "message": "Deployed commit abc1234 to staging",
    "level": "info",
    "attrs": {
      "commit": "abc1234",
      "environment": "staging"
    }
  }'
```

Returns `204 No Content` on success.

**Requirements:**

- `event` is required (snake_case identifier for the event type)
- `message` is optional (human-readable description)
- `level` is optional (defaults to `info`; valid values: `info`, `warn`, `error`)
- `attrs` is optional (arbitrary key-value metadata)

Custom events are written with `source: frontend` and appear in the `system` category. They are subject to the current audit mode -- if audit logging is `off`, the event is silently discarded.

## Deleting events

Remove audit events scoped by project, worktree, or time range. Purge operations are themselves logged as audit events (category: `system`, event: `audit_purged`).

```bash
# Delete all events for a project
curl -s -X DELETE 'http://localhost:8090/api/v1/audit?projectId=a1b2c3d4e5f6'

# Delete events older than a date
curl -s -X DELETE 'http://localhost:8090/api/v1/audit?until=2025-01-01T00:00:00Z'

# Delete events for a specific worktree
curl -s -X DELETE 'http://localhost:8090/api/v1/audit?projectId=a1b2c3d4e5f6&worktree=feat-auth'
```

The response includes the count of deleted events:

```json
{
  "deleted": 142
}
```

## Data lifecycle

### When a project is deleted

- **Audit logging on** (standard or detailed): cost data and audit events are preserved. The audit trail remains intact for historical review. You can still query and export events for deleted projects.
- **Audit logging off**: all associated costs and events are cleaned up with the project.

### Listing projects with audit data

To discover which projects have audit events recorded (including deleted projects when audit is on):

```bash
curl -s http://localhost:8090/api/v1/audit/projects | jq
```

Returns an array of project ID strings.

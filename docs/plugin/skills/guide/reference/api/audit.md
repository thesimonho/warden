<!-- GENERATED from docs/openapi/swagger.yaml — do not edit manually -->

# Audit API

All error responses return `{"error": "message", "code": "ERROR_CODE"}`.
## Delete audit events

- **Method:** `DELETE`
- **Path:** `/api/v1/audit`
- **Tags:** audit

Removes audit events matching the given filters. Supports scoping by project, worktree, and time range.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`deleted`**

  `integer`

**Example:**

```json
{
  "deleted": 1
}
```

##### Status: 500 Internal Server Error
---

## Get audit log

- **Method:** `GET`
- **Path:** `/api/v1/audit`
- **Tags:** audit

Returns audit-relevant events (sessions, tool uses, prompts, lifecycle) with optional filtering by project, worktree, category, and time range.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Array of:**

- **`agentType`**

  `string` — AgentType identifies the agent that produced this event (e.g. "claude-code", "codex").

- **`attrs`**

  `object` — Attrs carries structured key-value metadata.

- **`category`**

  `string` — Category is the audit category (session, agent, prompt, config, system). Computed at query time from the event name — not stored in the DB.

- **`containerName`**

  `string` — ContainerName is a snapshot of the container name at the time of the event.

- **`data`**

  `array` — Data carries the raw event payload (for agent events, preserves hook JSON).

  **Items:**

  `integer`

- **`event`**

  `string` — Event is a snake\_case identifier for the event type (e.g. "session\_start").

- **`id`**

  `integer` — ID is the database row identifier. Unique across all entries.

- **`level`**

  `string`, possible values: `"info", "warn", "error"` — Level is the severity of the entry (info, warn, error).

- **`msg`**

  `string` — Message is a human-readable description.

- **`projectId`**

  `string` — ProjectID is the deterministic project identifier (sha256 of host path, 12 hex chars).

- **`source`**

  `string`, possible values: `"agent", "backend", "frontend", "container", "external"` — Source identifies the origin layer (agent, backend, frontend, container).

- **`ts`**

  `string` — Timestamp is when the event occurred (ISO 8601 with milliseconds).

- **`worktree`**

  `string` — Worktree is the worktree ID (only for agent events).

**Example:**

```json
[
  {
    "agentType": "",
    "attrs": {},
    "category": "",
    "containerName": "",
    "data": [
      1
    ],
    "event": "",
    "id": 1,
    "level": "info",
    "msg": "",
    "projectId": "",
    "source": "agent",
    "ts": "",
    "worktree": ""
  }
]
```

##### Status: 500 Internal Server Error
---

## Post audit event

- **Method:** `POST`
- **Path:** `/api/v1/audit`
- **Tags:** audit

Writes a custom audit event. Integrators can set source, project scope, and structured data. Source defaults to "external" if omitted.

#### Request Body

##### Content-Type: application/json

**One of:**

- **`agentType`**

  `string` — AgentType scopes the event to an agent type (e.g. "claude-code", "codex"). Optional.

- **`attrs`**

  `object` — Attrs carries key-value metadata.

- **`data`**

  `array` — Data carries a raw JSON payload for structured event data.

  **Items:**

  `integer`

- **`event`**

  `string` — Event is a snake\_case identifier for the event type (e.g. "deployment\_started"). Required.

- **`level`**

  `string` — Level is the severity ("info", "warn", "error"). Defaults to "info" if omitted.

- **`message`**

  `string` — Message is a human-readable description.

- **`projectId`**

  `string` — ProjectID associates the event with a project. Optional.

- **`source`**

  `string` — Source identifies the origin of the event. Must be a valid AuditSource value. Defaults to "external" if omitted.

- **`worktree`**

  `string` — Worktree associates the event with a worktree. Optional.

**Example:**

```json
{}
```

#### Responses

##### Status: 204 No Content

##### Status: 400 Bad Request
##### Status: 500 Internal Server Error
---

## Export audit log

- **Method:** `GET`
- **Path:** `/api/v1/audit/export`
- **Tags:** audit

Downloads audit events as CSV or JSON for compliance review.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

`null`

**Example:**

```json
null
```

##### Status: 500 Internal Server Error
---

## List audit projects

- **Method:** `GET`
- **Path:** `/api/v1/audit/projects`
- **Tags:** audit

Returns distinct project IDs that have audit events recorded.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

**Array of:**

`string`

**Example:**

```json
[
  ""
]
```

##### Status: 500 Internal Server Error
---

## Get audit summary

- **Method:** `GET`
- **Path:** `/api/v1/audit/summary`
- **Tags:** audit

Returns aggregate statistics for audit events including session counts, tool usage, cost totals, and top tools.

#### Responses

##### Status: 200 OK

###### Content-Type: application/json

- **`timeRange`**

  `object` — TimeRange holds the earliest and latest event timestamps.

  - **`earliest`**

    `string`

  - **`latest`**

    `string`

- **`topTools`**

  `array` — TopTools lists the most frequently used tools with counts.

  **Items:**

  - **`count`**

    `integer`

  - **`name`**

    `string`

- **`totalCostUsd`**

  `number` — TotalCostUSD is the aggregate cost across all projects.

- **`totalPrompts`**

  `integer` — TotalPrompts is the number of user\_prompt events.

- **`totalSessions`**

  `integer` — TotalSessions is the number of session\_start events.

- **`totalToolUses`**

  `integer` — TotalToolUses is the number of tool\_use events.

- **`uniqueProjects`**

  `integer` — UniqueProjects is the number of distinct projects with events.

- **`uniqueWorktrees`**

  `integer` — UniqueWorktrees is the number of distinct worktrees with events.

**Example:**

```json
{
  "timeRange": {
    "earliest": "",
    "latest": ""
  },
  "topTools": [
    {
      "count": 1,
      "name": ""
    }
  ],
  "totalCostUsd": 1,
  "totalPrompts": 1,
  "totalSessions": 1,
  "totalToolUses": 1,
  "uniqueProjects": 1,
  "uniqueWorktrees": 1
}
```

##### Status: 500 Internal Server Error

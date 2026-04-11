# Settings

Warden settings are global server configuration that affects all projects. They control audit logging, budget enforcement, terminal behavior, and other server-wide defaults.

This page covers settings concepts and HTTP API patterns. See [api/settings.md](./api/settings.md) for full request/response field definitions.

## Reading settings

```bash
curl -s http://localhost:8090/api/v1/settings | jq
```

The response includes both configurable settings and read-only server information:

```json
{
  "version": "v0.5.2",
  "runtime": "go1.23.0",
  "claudeCodeVersion": "1.0.12",
  "codexVersion": "0.1.2",
  "workingDirectory": "/home/user",
  "auditLogMode": "off",
  "defaultProjectBudget": 0,
  "notificationsEnabled": true,
  "budgetActionWarn": true,
  "budgetActionStopWorktrees": false,
  "budgetActionStopContainer": false,
  "budgetActionPreventStart": false,
  "disconnectKey": "ctrl+d"
}
```

### Read-only fields

These fields are informational and cannot be changed via the API:

| Field               | Description                                                                                  |
| ------------------- | -------------------------------------------------------------------------------------------- |
| `version`           | Server build version (e.g. `"v0.5.2"`, `"dev"`)                                              |
| `runtime`           | Go runtime version                                                                           |
| `claudeCodeVersion` | Pinned Claude Code CLI version installed in containers                                       |
| `codexVersion`      | Pinned Codex CLI version installed in containers                                             |
| `workingDirectory`  | Server process working directory. Useful for development tooling that auto-creates projects. |

## Updating settings

Use `PUT` with a partial object -- only the fields you include are changed. Omitted fields retain their current values.

```bash
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{
    "auditLogMode": "standard"
  }'
```

The response indicates whether a server restart is needed:

```json
{
  "restartRequired": false
}
```

### Restart-required changes

Some settings changes take effect immediately; others require a server restart. The response always tells you:

| Setting                | Restart required? | Notes                                        |
| ---------------------- | ----------------- | -------------------------------------------- |
| `auditLogMode`         | No                | Synced to all running containers immediately |
| `notificationsEnabled` | No                | Takes effect immediately                     |
| `defaultProjectBudget` | No                | Applies on the next budget check             |
| `budgetAction*`        | No                | Applies on the next budget check             |
| `disconnectKey`        | No                | Applies to new terminal connections          |

## Available settings

### Audit log mode

Controls what events are written to the audit database. See [audit.md](./audit.md) for details on modes and categories.

```bash
# Enable standard audit logging
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"auditLogMode": "standard"}'

# Enable detailed audit logging
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"auditLogMode": "detailed"}'

# Disable audit logging
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"auditLogMode": "off"}'
```

When changed, the new mode is synced to all running containers immediately.

### Budget settings

Global defaults for cost enforcement. See [cost-budget.md](./cost-budget.md) for the full budget system.

```bash
# Set a $20 default budget with warn + stop worktrees
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{
    "defaultProjectBudget": 20.00,
    "budgetActionWarn": true,
    "budgetActionStopWorktrees": true,
    "budgetActionStopContainer": false,
    "budgetActionPreventStart": false
  }'
```

| Setting                     | Type    | Description                                                                                              |
| --------------------------- | ------- | -------------------------------------------------------------------------------------------------------- |
| `defaultProjectBudget`      | number  | Global budget default in USD. `0` disables budget enforcement for projects without a per-project budget. |
| `budgetActionWarn`          | boolean | Broadcast a warning SSE event and write an audit entry                                                   |
| `budgetActionStopWorktrees` | boolean | Kill all worktree processes when budget is exceeded                                                      |
| `budgetActionStopContainer` | boolean | Stop the container when budget is exceeded                                                               |
| `budgetActionPreventStart`  | boolean | Block container restarts for over-budget projects                                                        |

### Notifications

Controls whether the system tray (`warden-tray`) sends native desktop notifications when agents need attention (permission prompts, idle state, elicitation dialogs). The tray subscribes to the SSE event stream and fires OS-native notifications based on attention state changes. Clicking a notification opens the project dashboard with `?worktrees=wid1,wid2` query parameters, which auto-connects terminals for the worktrees that need attention.

The tray also shows a **Needs Attention** submenu listing projects that currently need user input. Each submenu item opens the project dashboard with a deep link to the attention worktrees.

```bash
# Enable desktop notifications
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"notificationsEnabled": true}'

# Disable desktop notifications
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"notificationsEnabled": false}'
```

| Setting                | Type    | Description                                                                                        |
| ---------------------- | ------- | -------------------------------------------------------------------------------------------------- |
| `notificationsEnabled` | boolean | When `true`, the system tray sends native desktop notifications for agent attention state changes. |

This setting is stored server-side and read by the tray via the settings API. It takes effect immediately -- no restart required.

### Disconnect key

The keyboard shortcut that disconnects the terminal viewer without killing the agent process. The agent continues running in the background.

```bash
curl -s -X PUT http://localhost:8090/api/v1/settings \
  -H "Content-Type: application/json" \
  -d '{"disconnectKey": "ctrl+d"}'
```

## Health check

Use the health endpoint to verify the server is running and ready to accept requests. Useful for readiness polling, load balancer checks, and detecting a running Warden instance.

```bash
curl -s http://localhost:8090/api/v1/health
```

Response:

```json
{
  "status": "ok",
  "version": "v0.5.2"
}
```

The response includes an `X-Warden: 1` header that can be used for instance detection without parsing the body.

### Polling for readiness

```bash
# Wait for Warden to become available
until curl -sf http://localhost:8090/api/v1/health > /dev/null 2>&1; do
  sleep 1
done
echo "Warden is ready"
```

## Shutdown

Request a graceful shutdown of the Warden server. The server broadcasts a `server_shutdown` SSE event so connected clients can react before the connection drops, then drains active connections.

```bash
curl -s -X POST http://localhost:8090/api/v1/shutdown
```

Response:

```json
{
  "status": "shutting down"
}
```

The `server_shutdown` SSE event is sent before the HTTP response, so SSE clients receive it while the connection is still open.

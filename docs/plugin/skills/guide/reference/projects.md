# Projects

A **project** is a workspace (local host directory or remote git repository) registered with Warden for running an AI coding agent in an isolated container. Projects are the top-level unit of organization -- every container, worktree, terminal, cost record, and audit event belongs to a project.

This page covers project identity, lifecycle, and common integration patterns using the HTTP API. See [api/projects.md](./api/projects.md) for full request/response field definitions.

## Project identity

A project is uniquely identified by a **compound primary key**: `(projectID, agentType)`.

- **`projectID`** is a deterministic 12-character hex string computed from the SHA-256 of the resolved absolute host path (local projects) or normalized clone URL (remote projects). The same directory or URL always produces the same ID, regardless of symlinks, trailing slashes, or `.git` suffixes.
- **`agentType`** is either `"claude-code"` or `"codex"`.

This compound key means the same directory (or repository) can have two independent projects -- one for each agent type -- each with its own container, cost history, and audit trail.

All scoped API routes use the pattern:

```
/api/v1/projects/{projectId}/{agentType}/...
```

## Creating a project

Adding a project is a **two-step flow**: register the project, then create its container.

### Step 1: Register the project

**Local project** (bind-mount a host directory):

```bash
curl -s -X POST http://localhost:8090/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "projectPath": "/home/user/my-app",
    "name": "my-app"
  }'
```

**Remote project** (clone a git repository):

```bash
curl -s -X POST http://localhost:8090/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{
    "cloneURL": "https://github.com/org/my-app.git",
    "name": "my-app"
  }'
```

Set `"temporary": true` to make the workspace ephemeral (lost on container recreate). Otherwise, the cloned workspace is stored in a Docker volume.

The response includes the computed `projectId` and `agentType`:

```json
{
  "projectId": "a1b2c3d4e5f6",
  "agentType": "claude-code",
  "name": "my-app",
  "containerId": ""
}
```

**Idempotent behavior:** Adding the same host path or clone URL again returns the existing project rather than creating a duplicate. The `agentType` is determined by the server's configured agent type (set via the `WARDEN_AGENT_TYPE` environment variable, defaults to `claude-code`).

### Step 2: Create the container

See [containers.md](./containers.md) for the container creation step.

## Listing projects

The list endpoint returns all projects enriched with live state from Docker, the agent, and the cost system:

```bash
curl -s http://localhost:8090/api/v1/projects | jq
```

Each project object includes:

- **Container state** (`state`, `status`, `hasContainer`) -- whether the container exists and is running
- **Agent state** (`agentStatus`, `needsInput`, `notificationType`, `attentionWorktreeIDs`) -- what the agent is doing right now
- **Cost data** (`totalCost`, `costBudget`, `isEstimatedCost`) -- spend tracking
- **Worktree counts** (`activeWorktreeCount`, `isGitRepo`) -- how many worktrees have connected terminals
- **Configuration** (`image`, `networkMode`, `allowedDomains`, `skipPermissions`) -- current settings

This is the primary endpoint for building a project dashboard. For real-time updates without polling, subscribe to the SSE event stream at `GET /api/v1/events` -- it pushes `project_state` events whenever cost, attention, or container state changes.

## Removing a project

Removing a project deletes the tracking record from Warden's database. It does **not** stop or delete the container -- do that separately if needed.

```bash
curl -s -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code
```

```json
{
  "projectId": "a1b2c3d4e5f6",
  "agentType": "claude-code",
  "name": "my-app",
  "containerId": "abc123..."
}
```

To fully clean up, delete the container first (see [containers.md](./containers.md)), then remove the project.

## Stopping a project

Gracefully stops the container. All worktree processes (tmux sessions, running agents) are terminated. Cost data is captured before shutdown.

```bash
curl -s -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/stop
```

## Restarting a project

Restarts a stopped container. Warden performs two validation checks before allowing the restart:

1. **Mount validation** -- all bind mount host paths must still exist. If any source has been moved or deleted, the restart fails with a `409` and error code `STALE_MOUNTS`.
2. **Budget check** -- if the project's cost exceeds its budget and the `preventStart` enforcement action is enabled, the restart fails with a `403` and error code `BUDGET_EXCEEDED`.

```bash
curl -s -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/restart
```

Handle the error codes in your integration:

```bash
# Check for stale mounts
curl -s -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/restart
# 409: {"error": "bind mount sources no longer exist: /home/user/.ssh", "code": "STALE_MOUNTS"}

# Check for budget exceeded
# 403: {"error": "cost budget exceeded", "code": "BUDGET_EXCEEDED"}
```

## Project templates

A `.warden.json` file in a project directory provides version-controlled default configuration. When a team commits this file, every developer gets the same container settings (image, network mode, runtimes, budget) without manual setup.

### Reading a template

Read a `.warden.json` from a specific path:

```bash
curl -s "http://localhost:8090/api/v1/template?path=/home/user/my-app/.warden.json"
```

### Getting defaults with template

The defaults endpoint returns server-detected defaults (home directory, auto-detected mounts, available runtimes) merged with any `.warden.json` template found in the project directory:

```bash
curl -s "http://localhost:8090/api/v1/defaults?path=/home/user/my-app&agentType=claude-code"
```

This is the recommended way to pre-populate a "create project" form. The response includes:

- `homeDir` -- host home directory
- `mounts` -- auto-detected bind mounts (with `required` flags)
- `runtimes` -- available language runtimes with `detected` flags based on project marker files
- `template` -- parsed `.warden.json` values, if present

See [api/host.md](./api/host.md) for the full response shape.

## Resetting costs

Clear all cost history for a project:

```bash
curl -s -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/costs
# 204 No Content
```

## Purging audit history

Remove all audit events for a project:

```bash
curl -s -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/audit
```

## Host desktop integration

Warden provides two endpoints for launching host desktop applications from the web UI or a custom frontend. Both accept a JSON body with a single `path` field (an absolute host directory path) and return `204 No Content` on success.

### Reveal in file manager

Opens the given directory in the system file manager (Finder, Nautilus, Explorer, etc.).

```bash
curl -s -X POST http://localhost:8090/api/v1/filesystem/reveal \
  -H "Content-Type: application/json" \
  -d '{"path": "/home/user/my-app"}'
# 204 No Content on success
```

### Open in code editor

Opens the given directory in the user's preferred code editor (detected via `$EDITOR`, `$VISUAL`, or common editor binaries such as `code`, `cursor`, `zed`).

```bash
curl -s -X POST http://localhost:8090/api/v1/filesystem/editor \
  -H "Content-Type: application/json" \
  -d '{"path": "/home/user/my-app"}'
# 204 No Content on success
```

Error responses:

| HTTP Status | Error Code     | Meaning                                             |
| ----------- | -------------- | --------------------------------------------------- |
| 400         | `INVALID_PATH` | Path is not an absolute path or fails safety checks |
| 404         | `NOT_FOUND`    | Path does not exist on the host                     |
| 422         | `NO_EDITOR`    | No code editor was found on the host                |
| 500         | (varies)       | Failed to launch the editor                         |

Both endpoints are host-only — they interact with the host machine running Warden, not the container. See [api/host.md](./api/host.md) for full field definitions.

## Common patterns

### Full create flow

A typical integration creates a project in three steps:

```bash
# 1. Get defaults (detects runtimes, reads .warden.json template)
DEFAULTS=$(curl -s "http://localhost:8090/api/v1/defaults?path=/home/user/my-app&agentType=claude-code")

# 2. Register the project
PROJECT=$(curl -s -X POST http://localhost:8090/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"projectPath": "/home/user/my-app", "name": "my-app"}')

PROJECT_ID=$(echo "$PROJECT" | jq -r '.projectId')
AGENT_TYPE=$(echo "$PROJECT" | jq -r '.agentType')

# 3. Create the container (see containers.md for full config options)
curl -s -X POST "http://localhost:8090/api/v1/projects/${PROJECT_ID}/${AGENT_TYPE}/container" \
  -H "Content-Type: application/json" \
  -d '{
    "projectPath": "/home/user/my-app",
    "name": "my-app",
    "envVars": {"ANTHROPIC_API_KEY": "sk-ant-..."},
    "networkMode": "restricted",
    "enabledRuntimes": ["node", "python"]
  }'

# 4. Connect a terminal (see worktrees/terminals documentation)
```

### Polling vs SSE for state

For dashboards, prefer the SSE event stream over polling the list endpoint:

```bash
# SSE stream (recommended for real-time UIs)
curl -N http://localhost:8090/api/v1/events

# Polling (simpler, acceptable for batch workflows)
curl -s http://localhost:8090/api/v1/projects
```

The SSE stream pushes `project_state` events with the same shape as list items, so you can update your UI incrementally without re-fetching the full list.

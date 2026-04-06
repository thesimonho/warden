# Containers

A **container** is a Docker container created for a project. There is exactly one container per `(projectID, agentType)` pair. The container runs the AI coding agent (Claude Code or Codex) in an isolated environment with configurable networking, mounts, environment variables, and language runtimes.

This page covers container creation, configuration, updates, and lifecycle management via the HTTP API. See [api/containers.md](./api/containers.md) for full request/response field definitions.

## Creating a container

Container creation is always the second step after registering a project (see [projects.md](./projects.md)). The `POST` endpoint accepts a `CreateContainerRequest` -- the most complex request body in the API.

```bash
curl -s -X POST http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "projectPath": "/home/user/my-app",
    "image": "ghcr.io/thesimonho/warden:latest",
    "agentType": "claude-code",
    "networkMode": "restricted",
    "allowedDomains": ["api.anthropic.com", "github.com", "*.githubusercontent.com"],
    "envVars": {
      "ANTHROPIC_API_KEY": "sk-ant-...",
      "GIT_AUTHOR_NAME": "Jane Dev",
      "GIT_AUTHOR_EMAIL": "jane@example.com"
    },
    "mounts": [
      {
        "hostPath": "/home/user/.claude",
        "containerPath": "/home/warden/.claude",
        "readOnly": false
      },
      {
        "hostPath": "/home/user/.ssh",
        "containerPath": "/home/warden/.ssh",
        "readOnly": true
      }
    ],
    "enabledRuntimes": ["node", "python", "go"],
    "enabledAccessItems": ["git", "ssh"],
    "costBudget": 50.00,
    "skipPermissions": false
  }'
```

Response (201 Created):

```json
{
  "projectId": "a1b2c3d4e5f6",
  "agentType": "claude-code",
  "name": "my-app",
  "containerId": "sha256:abc123...",
  "recreated": false
}
```

### Container image

Defaults to `ghcr.io/thesimonho/warden:latest` -- an Ubuntu 24.04 image with Claude Code, Codex, and all terminal infrastructure pre-installed. Custom images are supported but must include the required infrastructure (tmux, terminal scripts). Use the validate endpoint to check.

### Environment variables

Key-value pairs injected into the container. Common variables:

| Variable | Purpose |
|---|---|
| `ANTHROPIC_API_KEY` | Required for Claude Code (unless using subscription login) |
| `OPENAI_API_KEY` | Required for Codex (unless using subscription login) |
| `GITHUB_TOKEN` | GitHub API access |
| `GIT_AUTHOR_NAME` | Override git commit author name |
| `GIT_AUTHOR_EMAIL` | Override git commit author email |

Environment variables persist across container restarts.

### Bind mounts

Each mount maps a host path into the container:

- `hostPath` -- absolute path on the host machine
- `containerPath` -- absolute path inside the container
- `readOnly` -- whether the container can write to it

The agent config directory (`~/.claude` for Claude Code, `~/.codex` for Codex) is always mounted and required for authentication and session tracking. The defaults endpoint (`GET /api/v1/defaults`) returns this as a mount with `required: true` -- do not allow users to remove it.

**Mount validation:** Warden resolves symlinks and validates that host paths exist at container creation time. After creation, if a mount source is moved or deleted, Warden blocks restarts with a `STALE_MOUNTS` error (409). Fix or remove the stale mount via an update before restarting.

### Network mode

Controls outbound network access:

| Mode | Behavior |
|---|---|
| `"full"` | Unrestricted outbound access |
| `"restricted"` | Only `allowedDomains` are reachable (enforced via iptables) |
| `"none"` | Fully air-gapped, no network access |

When using `restricted` mode, populate `allowedDomains` with the domains the agent needs. Each enabled runtime automatically adds its package registry domains (e.g., `registry.npmjs.org` for Node.js).

### Runtimes

Language runtimes installable in the container. Each runtime installs the base toolchain and opens the minimum network surface for its package registry.

| Runtime ID | Language | Always enabled |
|---|---|---|
| `node` | Node.js | Yes (required for MCP servers) |
| `python` | Python | No |
| `go` | Go | No |
| `rust` | Rust | No |
| `ruby` | Ruby | No |
| `lua` | Lua | No |

Runtimes are auto-detected from project marker files (e.g., `go.mod` for Go, `pyproject.toml` for Python). The defaults endpoint returns detection results so you can pre-select them.

### Cost budget

A per-project spending limit in USD. Set to `0` to use the global default budget. When costs exceed the budget, Warden can warn, stop worktrees, stop the container, or prevent restarts depending on the enforcement settings.

### Skip permissions

When `true`, the agent starts in fully autonomous mode (`--dangerously-skip-permissions` for Claude Code, `--dangerously-bypass-approvals-and-sandbox` for Codex). Only enable this for trusted automation workflows.

### Access items

IDs of access items to enable (e.g., `["git", "ssh"]`). Access items resolve credentials from the host and inject them into the container at creation time. See the access items documentation for available items.

## Updating a container

The update endpoint accepts the same body shape as create. Warden classifies changes into two categories:

- **Lightweight changes** (`costBudget`, `skipPermissions`, `allowedDomains`) -- applied in-place without restarting
- **Structural changes** (`image`, `mounts`, `envVars`, `networkMode`, `enabledRuntimes`, `enabledAccessItems`) -- trigger a full container recreation

The response includes `recreated: true` or `recreated: false` so your integration knows what happened.

```bash
# Update budget (lightweight, no recreation)
curl -s -X PUT http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d '{
    "name": "my-app",
    "projectPath": "/home/user/my-app",
    "costBudget": 100.00,
    "networkMode": "restricted",
    "allowedDomains": ["api.anthropic.com", "github.com"],
    "envVars": {"ANTHROPIC_API_KEY": "sk-ant-..."},
    "mounts": [
      {"hostPath": "/home/user/.claude", "containerPath": "/home/warden/.claude", "readOnly": false}
    ],
    "enabledRuntimes": ["node"],
    "skipPermissions": false
  }'
```

```json
{
  "projectId": "a1b2c3d4e5f6",
  "agentType": "claude-code",
  "name": "my-app",
  "containerId": "sha256:def456...",
  "recreated": false
}
```

**Important:** The update body replaces the entire configuration. Always send the full desired state, not just the fields you want to change. Read the current config first if you only need to modify one field.

## Deleting a container

Stops and permanently removes the container. The project record remains in Warden's database -- only the container is destroyed. Worktree state, terminal processes, and session data inside the container are lost.

```bash
curl -s -X DELETE http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container
```

```json
{
  "projectId": "a1b2c3d4e5f6",
  "agentType": "claude-code",
  "name": "my-app",
  "containerId": "sha256:abc123...",
  "recreated": false
}
```

## Inspecting configuration

Retrieve the current editable configuration for a container:

```bash
curl -s http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container/config | jq
```

```json
{
  "name": "my-app",
  "projectPath": "/home/user/my-app",
  "agentType": "claude-code",
  "image": "ghcr.io/thesimonho/warden:latest",
  "networkMode": "restricted",
  "allowedDomains": ["api.anthropic.com", "github.com"],
  "envVars": {"GIT_AUTHOR_NAME": "Jane Dev"},
  "mounts": [
    {"hostPath": "/home/user/.claude", "containerPath": "/home/warden/.claude", "readOnly": false}
  ],
  "enabledRuntimes": ["node", "python"],
  "enabledAccessItems": ["git"],
  "costBudget": 50.0,
  "skipPermissions": false
}
```

This is useful for building "edit project" forms or for read-modify-write update flows.

## Validating infrastructure

Check whether a running container has the required Warden terminal infrastructure installed:

```bash
curl -s http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container/validate | jq
```

```json
{
  "valid": true,
  "missing": []
}
```

If `valid` is `false`, the `missing` array lists components that need to be installed (e.g., `tmux`, `create-terminal.sh`). This is primarily useful when using custom container images to verify they include the required infrastructure.

## Common patterns

### Read-modify-write update

To change a single setting without affecting others:

```bash
# 1. Read current config
CONFIG=$(curl -s http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container/config)

# 2. Modify the field you want (e.g., bump budget)
UPDATED=$(echo "$CONFIG" | jq '.costBudget = 100.0')

# 3. Write it back
curl -s -X PUT http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d "$UPDATED"
```

### Detecting recreation

After an update, check `recreated` to decide whether your integration needs to reconnect terminals:

```bash
RESULT=$(curl -s -X PUT http://localhost:8090/api/v1/projects/a1b2c3d4e5f6/claude-code/container \
  -H "Content-Type: application/json" \
  -d "$UPDATED_CONFIG")

if [ "$(echo "$RESULT" | jq -r '.recreated')" = "true" ]; then
  echo "Container was recreated -- reconnect terminals"
fi
```

### Error handling

All error responses follow the same shape:

```json
{"error": "human-readable message", "code": "ERROR_CODE"}
```

Key error codes for container operations:

| HTTP Status | Error Code | Meaning |
|---|---|---|
| 400 | (varies) | Invalid request body or missing required fields |
| 404 | (varies) | Project or container not found |
| 409 | `STALE_MOUNTS` | Bind mount host paths no longer exist (on restart) |
| 409 | (varies) | Container name already in use (on create) |
| 403 | `BUDGET_EXCEEDED` | Cost budget exceeded with `preventStart` enabled (on restart) |

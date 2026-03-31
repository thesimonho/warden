---
title: Projects
description: Manage workspaces, containers, and per-project configuration.
---

A **Project** is a host directory paired with a container that runs an AI coding agent against it. Each project gets its own isolated container with configurable image, environment, mounts, network policy, and cost budget. Projects are the top-level unit of organization in Warden.

## Creating a Project

Add a project by providing:

- **Agent Type** — the CLI agent to run: **Claude Code** (Anthropic) or **Codex** (OpenAI). This is selected first and cannot be changed after creation.
- **Name** — a display name (also determines the workspace path inside the container: `/home/dev/<name>`)
- **Host Path** — the absolute path to the directory on your machine

If you add the same path again, Warden returns the existing project instead of creating a duplicate.

## Container Configuration

Each project's container is configured at creation time. You can update the configuration later — Warden recreates the container with the new settings.

### Image

The container image to use. Defaults to `ghcr.io/thesimonho/warden:latest` (Ubuntu 24.04 with Claude Code, Codex, and all terminal infrastructure pre-installed).

To use a custom image, see [Custom Images](/warden/guide/devcontainers/).

### Environment Variables

Key-value pairs injected into the container. Useful for API keys, git identity, and tool configuration:

- `ANTHROPIC_API_KEY` — required for Claude Code projects (unless using subscription login)
- `OPENAI_API_KEY` — required for Codex projects (unless using subscription login)
- `GITHUB_TOKEN` — for GitHub API access
- `GIT_AUTHOR_NAME`, `GIT_AUTHOR_EMAIL` — override git identity

Environment variables persist across container restarts and are available in every terminal.

### Bind Mounts

Additional host directories or files to mount into the container. Each mount specifies:

- **Host path** — absolute path on your machine
- **Container path** — where it appears inside the container
- **Read-only** — whether the container can write to it (default: read-write)

Warden validates that host paths exist and resolves symlinks before creating the container. If a mount source is moved or deleted after creation, Warden detects the stale mount and blocks restarts until you fix it.

:::caution
Mount sensitive files (credentials, config) as read-only to prevent the container from modifying them. For credential injection, prefer [Access Items](/warden/features/access/) over raw bind mounts.
:::

### Network Mode

Controls outbound network access from the container — choose between unrestricted, a domain allowlist, or fully air-gapped. See [Network Isolation](/warden/features/network/) for details.

### Cost Budget

A per-project spending limit in USD. When the total cost exceeds the budget, Warden takes the enforcement actions configured in [Settings](/warden/features/cost-budget/#enforcement-actions). Set to 0 (or leave blank) to use the global default budget.

See [Cost & Budget](/warden/features/cost-budget/) for the full cost tracking system.

### Access Items

Select which [Access Items](/warden/features/access/) to enable for this project. Enabled items have their credentials resolved and injected at container creation time.

### Skip Permissions

When enabled, terminals start the agent in fully autonomous mode, bypassing tool approval prompts. Claude Code uses `--dangerously-skip-permissions`; Codex uses `--dangerously-bypass-approvals-and-sandbox`. Useful for trusted automation workflows.

:::caution
Skipping permissions gives the agent unrestricted access to the tools available in the container. Only enable this for projects where you trust the prompts being sent.
:::

## Container Lifecycle

| Action | What happens |
|--------|-------------|
| **Create** | Pulls the image (if needed), resolves [Access Items](/warden/features/access/), starts the container with all configured settings |
| **Stop** | Captures latest cost data, then stops the container gracefully. All worktree processes are terminated. |
| **Restart** | Validates bind mount sources still exist and checks cost budget before restarting. Blocks if mounts are stale or budget is exceeded with `preventStart` enabled. |
| **Delete** | Stops and removes the container. The project record remains — only the container is destroyed. |
| **Update** | Recreates the container with new configuration. Worktree state on disk is preserved. |

## Process Hardening

Every container is automatically hardened — no configuration needed:

- **Capability dropping** — all Linux capabilities dropped, only the minimum required set re-added
- **Seccomp profile** — blocks dangerous syscalls (kernel module loading, filesystem mounting, etc.) while allowing standard dev tooling
- **No new privileges** — prevents privilege escalation via setuid/setgid binaries
- **PID limit** — 512 max processes per container to prevent fork bombs

## For Developers

### HTTP API

| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/v1/projects` | List all projects with status and cost |
| `POST` | `/api/v1/projects` | Add a project |
| `DELETE` | `/api/v1/projects/{projectId}` | Remove a project |
| `POST` | `/api/v1/projects/{projectId}/stop` | Stop container |
| `POST` | `/api/v1/projects/{projectId}/restart` | Restart container |
| `POST` | `/api/v1/projects/{projectId}/container` | Create container with config |
| `PUT` | `/api/v1/projects/{projectId}/container` | Update container config |
| `DELETE` | `/api/v1/projects/{projectId}/container` | Delete container |
| `GET` | `/api/v1/projects/{projectId}/container/config` | Inspect current config |
| `GET` | `/api/v1/projects/{projectId}/container/validate` | Validate infrastructure |

### Go Client

```go
c := client.New("http://localhost:8090")

// List all projects
projects, _ := c.ListProjects(ctx)

// Add a project (agentType is set when creating the container)
result, _ := c.AddProject(ctx, "my-project", "/home/user/code/my-project")

// Create container with configuration
result, _ := c.CreateContainer(ctx, projectID, engine.CreateContainerRequest{
    Image:    "ghcr.io/thesimonho/warden:latest",
    EnvVars:  map[string]string{"ANTHROPIC_API_KEY": os.Getenv("ANTHROPIC_API_KEY")},
    Mounts:   []engine.Mount{{HostPath: "/home/user/.claude", ContainerPath: "/home/dev/.claude"}},
    NetworkMode: "restricted",
    AllowedDomains: []string{"github.com", "npmjs.org"},
})

// Stop, restart, delete
c.StopProject(ctx, projectID)
c.RestartProject(ctx, projectID)
c.DeleteContainer(ctx, projectID)
```

### Go Library

When using Warden as a Go library, project operations are available on the `service.Service` type:

```go
app, _ := warden.New(warden.Options{})

// Add and configure a project
result, _ := app.Service.AddProject("my-project", "/home/user/code/my-project")

// Create container (same CreateContainerRequest as the client)
containerResult, _ := app.Service.CreateContainer(ctx, project, engine.CreateContainerRequest{...})
```

See the [Go Packages](/warden/reference/go/) reference for full API documentation.

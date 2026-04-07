---
title: Projects
description: Manage workspaces, containers, and per-project configuration.
---

A **Project** is a workspace backed by either a local host directory or a remote git repository, paired with a container that runs an AI coding agent against it. Each project gets its own isolated container with configurable image, environment, mounts, network policy, and cost budget. Projects are the top-level unit of organization in Warden.

## Creating a Project

Add a project by providing:

- **Agent Type** — the CLI agent to run: **Claude Code** (Anthropic) or **Codex** (OpenAI). This is selected first and cannot be changed after creation.
- **Name** — a display name (also determines the workspace path inside the container: `/home/warden/<name>`)
- **Source** — either **Local** or **Remote**:
  - **Local** — provide the absolute **Host Path** to the directory on your machine. Warden bind-mounts it into the container.
  - **Remote** — provide a **Clone URL** (HTTPS or SSH). The repository is cloned inside the container on first boot. By default, the cloned workspace is persisted in a Docker volume that survives container recreation. Check **Temporary** to use the container's writable layer instead (workspace is lost on recreate).

If you add the same path or URL again, Warden returns the existing project instead of creating a duplicate.

## Container Configuration

Each project's container is configured at creation time. You can update the configuration later — lightweight changes (budget, skip permissions, allowed domains) are applied in-place, while structural changes (image, mounts, env vars, network mode) recreate the container.

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

Host directories or files to mount into the container. Each mount specifies:

- **Host path** — absolute path on your machine
- **Container path** — where it appears inside the container
- **Read-only** — whether the container can write to it (default: read-write)

The agent config directory (`~/.claude` or `~/.codex`) is always mounted and cannot be removed — the agent needs it for authentication and session tracking. You can change the host path if your config lives in a non-standard location. Additional mounts are optional.

Warden validates that host paths exist and resolves symlinks before creating the container. If a mount source is moved or deleted after creation, Warden detects the stale mount and blocks restarts until you fix it.

:::note[Nix Home Manager / GNU Stow]
If your config files are symlinks managed by Nix or Stow, Warden automatically dereferences them at container startup so agents can write config changes (e.g. model selection via `/model`). Changes made inside the container are container-local — your host's managed config is not modified. Recreate the container to pick up host config changes.
:::

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

| Action      | What happens                                                                                                                                                     |
| ----------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **Create**  | Pulls the image (if needed), resolves [Access Items](/warden/features/access/), starts the container with all configured settings                                |
| **Stop**    | Captures latest cost data, then stops the container gracefully. All worktree processes are terminated.                                                           |
| **Restart** | Validates bind mount sources still exist and checks cost budget before restarting. Blocks if mounts are stale or budget is exceeded with `preventStart` enabled. |
| **Delete**  | Stops and removes the container. The project record remains — only the container is destroyed.                                                                   |
| **Update**  | Applies configuration changes. Lightweight settings (budget, skip permissions, domains) update in-place; structural changes recreate the container.              |

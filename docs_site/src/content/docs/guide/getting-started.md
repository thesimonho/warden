---
title: Getting Started
description: Get up and running with Warden in under a minute.
---

## Prerequisites

- [Git](https://git-scm.com/downloads) — required for worktree support
- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation)
- An agent CLI account — [Claude Code](https://docs.anthropic.com/en/docs/agents-and-tools/claude-code/overview) (Anthropic) or [Codex](https://github.com/openai/codex) (OpenAI)

## Choose your binary

| I want to...              | Download         |
| ------------------------- | ---------------- |
| Use the web dashboard     | `warden-desktop` |
| Use the terminal UI       | `warden-tui`     |
| Integrate into my own app | `warden`         |

Download the binary for your platform from the [releases page](https://github.com/thesimonho/warden/releases). See [Installation](../installation/) for platform-specific instructions.

## Run it

```bash
# Web dashboard — opens in your browser at 127.0.0.1:8090
./warden-desktop

# Or TUI — opens in your terminal
./warden-tui

# Headless server for developer integration
./warden
```

The container image is pulled automatically from `ghcr.io/thesimonho/warden` on first use.

## Create your first project

When creating a project, the first field is the **agent type** — choose between Claude Code and Codex. Each project is locked to one agent at creation time.

Fill in the project name and host path, configure environment variables and other settings, then create. Warden pulls the container image (if needed) and starts the project.

## Authentication

### Claude Code

**API key:** Pass `ANTHROPIC_API_KEY` as an environment variable when creating the container.

**Subscription login:** Open the terminal and run `claude` — it prompts you to authenticate via browser. Authentication persists across restarts.

### Codex

**API key:** Pass `OPENAI_API_KEY` as an environment variable when creating the container.

**Subscription login:** Open the terminal and run `codex` — it prompts you to authenticate. Authentication persists across restarts.

## Agent config directories

Share your host's agent config directory into containers by setting bind mounts when creating a project. Different projects can use different config directories.

- **Claude Code:** `~/.claude` (skills, MCP plugins, settings)
- **Codex:** `~/.codex` (agent configuration)

## Environment variables

Any env var set at container creation is forwarded into the shell session — useful for `GITHUB_TOKEN`, `GIT_AUTHOR_NAME`, etc.

## Next steps

Explore Warden's feature set:

- [Projects](/warden/features/projects/) — container configuration, environment, mounts, and lifecycle
- [Worktrees & Terminals](/warden/features/worktrees/) — isolated workspaces with persistent terminal sessions
- [Access](/warden/features/access/) — credential passthrough from host to container
- [Network Isolation](/warden/features/network/) — control outbound traffic (full, restricted, or air-gapped)
- [Cost & Budget](/warden/features/cost-budget/) — per-project spending limits and enforcement
- [Audit Logging](/warden/features/audit/) — event logging for monitoring and compliance
- [Custom Images](../devcontainers/) — bring your own image (Dockerfile, devcontainer feature, or fully custom)
- [Integration Paths](../../integration/paths/) — embed Warden in your own application

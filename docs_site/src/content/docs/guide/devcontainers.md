---
title: Custom Images
description: Use your own container image with Warden.
---

Warden supports two paths for setting up your project's container environment:

1. **Use the Warden base image** — works out of the box with zero setup.
2. **Bring your own image** — built however you want (custom Dockerfile, devcontainer feature, Nix, etc.), as long as it includes Warden's terminal infrastructure.

This guide covers path 2.

## Why bring your own image?

- You need specific language runtimes, tools, or dependencies pre-installed.
- Your team already uses devcontainers and you want consistency.
- You need a different base OS (e.g., not Ubuntu 24.04).
- You want reproducible, CI-built images.

## Option A: Custom Dockerfile

Extend the Warden base image with your own tools:

```dockerfile
FROM ghcr.io/thesimonho/warden

USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3 \
    python3-pip \
    nodejs \
    npm \
    && rm -rf /var/lib/apt/lists/*
USER warden
```

Build and use it:

```bash
docker build -t my-warden-image .
```

Then select `my-warden-image` as the image when creating a project in Warden.

:::note
Always switch back to `USER warden` at the end of your Dockerfile. Warden's terminal infrastructure runs as the `warden` user.
:::

## Option B: Devcontainer feature

If you use [devcontainers](https://containers.dev/), add the Warden feature to your `.devcontainer/devcontainer.json`. This bakes Warden's terminal infrastructure (abduco, Claude Code CLI, hooks, network isolation) into whatever image your devcontainer config produces.

### Starter devcontainer.json

```json
{
  "name": "My Project",
  "image": "mcr.microsoft.com/devcontainers/base:ubuntu-24.04",
  "features": {
    "ghcr.io/thesimonho/warden/session-tools:1": {},
    "ghcr.io/devcontainers/features/node:1": {
      "version": "22"
    },
    "ghcr.io/devcontainers/features/go:1": {
      "version": "1.23"
    }
  },
  "postCreateCommand": "npm install"
}
```

Build the image with any devcontainer-compatible tool:

```bash
devcontainer build --workspace-folder . --image-name my-project:latest
```

Then select `my-project:latest` as the image when creating a project in Warden.

### What the feature installs

- **abduco** — terminal session manager for persistent sessions across disconnects
- **gosu** — lightweight privilege drop for the container entrypoint
- **Claude Code CLI** — the AI coding agent
- **GitHub CLI** — for `gh` commands inside the container
- **Node.js LTS** — needed for npx (MCP servers, etc.)
- **Terminal lifecycle scripts** — entrypoint, session creation, disconnect handling, process cleanup
- **Attention tracking hooks** — Claude Code hooks for real-time status monitoring
- **Network isolation tools** — iptables-based network policy enforcement
- **`warden` user** — non-root user for running terminals

All tools are installed idempotently — running the feature on an image that already has some of these tools is safe.

## Option C: Fully custom base image

If you need a completely different base image (Alpine, Fedora, a corporate base, etc.), you have two choices:

1. **Use the devcontainer feature** (Option B) — it works on any Debian/Ubuntu-based image.
2. **Manually install Warden's infrastructure** — copy the patterns from `container/scripts/install-tools.sh` in the Warden repo. The key requirements are: gosu, abduco, Claude Code CLI, the entrypoint and terminal lifecycle scripts, and a `warden` non-root user. See the required binaries below.

## Required Binaries

When you create a container, Warden validates that these binaries exist at `/usr/local/bin/`:

| Binary | Purpose |
|--------|---------|
| `gosu` | Privilege drop in entrypoint |
| `entrypoint.sh` | Root-phase setup (UID remapping, network isolation) |
| `user-entrypoint.sh` | User-phase setup (env forwarding, git/ssh config) |
| `create-terminal.sh` | Start abduco session + Claude Code |
| `disconnect-terminal.sh` | Clean disconnect (kill viewer, keep abduco) |
| `kill-worktree.sh` | Kill all processes for a worktree |

Options A and B install all of these automatically. Option C requires you to provide them.

## Which approach to use?

| Approach | Best for | Complexity |
| --- | --- | --- |
| Extend base image (Option A) | Adding a few packages on top of Ubuntu 24.04 | Low |
| Devcontainer feature (Option B) | Teams already using devcontainers, or needing a different base image | Medium |
| Fully custom (Option C) | Non-Debian base images or corporate environments | High |

---
title: Getting Started
description: Get up and running with Warden in under a minute.
---

## Prerequisites

- [Docker](https://docs.docker.com/get-docker/) or [Podman](https://podman.io/docs/installation)

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

## Authentication

**API key:** Pass `ANTHROPIC_API_KEY` as an environment variable when creating the container.

**Subscription login:** Open the terminal and run `claude` — it prompts you to authenticate via browser. Session persists across restarts.

## Claude config directory

Share your host's `~/.claude` into containers (skills, MCP plugins, settings) by setting the bind mounts when creating a container. Different projects can use different config directories.

## Environment variables

Any env var set at container creation is forwarded into the shell session — useful for `GITHUB_TOKEN`, `GIT_AUTHOR_NAME`, etc.

## Process hardening

Every container is automatically hardened with three security layers:

- **Capability dropping** — all Linux capabilities are dropped, then only the minimum required set is re-added (e.g. CHOWN for the entrypoint, SETUID for user switching, NET_ADMIN for network isolation). Dangerous capabilities like SETPCAP, MKNOD, and SETFCAP are never granted.
- **Seccomp profile** — a custom syscall filter blocks dangerous operations (kernel module loading, filesystem mounting, BPF program loading, etc.) while allowing all standard dev tooling.
- **No new privileges** — prevents privilege escalation via setuid/setgid binaries inside the container.

These are applied unconditionally to every container. No configuration needed.

## Network access controls

| Mode           | Description                                                                          |
| -------------- | ------------------------------------------------------------------------------------ |
| **Full**       | Unrestricted outbound access (default).                                              |
| **Restricted** | Outbound traffic limited to a configurable domain allowlist via iptables OUTPUT rules.|
| **None**       | All outbound traffic blocked. Only loopback and established connections allowed.     |

Enforced via iptables inside the container before any user code runs. The `NET_ADMIN` capability is only added to containers using restricted or none mode.

**Limitations:** Domain IPs are resolved once at container start (restart to re-resolve). Restricted/none modes may not work with rootless Podman depending on your configuration.

## Audit and event logging

Warden provides unified event logging for monitoring, auditing, and compliance. Enable it from the Settings dialog (web dashboard, TUI, or Go library).

### Event log modes

| Mode      | Description                                                                   |
| --------- | ----------------------------------------------------------------------------- |
| **Off**   | No events logged. Audit page is hidden. (default)                             |
| **Standard** | Core events only — sessions, terminal lifecycle, user prompts, cost/stop.   |
| **Detailed** | Everything above plus tool use, permissions, subagent activity, config changes. Higher volume, enables full audit dashboard. |

### Audit features

- **Activity timeline** — stacked bar chart with time range selection. Visualize event distribution by source (agent, backend, frontend, container).
- **Summary dashboard** — session count, tool uses, prompts, total cost, unique projects/worktrees, top tools.
- **Filters** — query by category (Sessions, Agent, Prompts, Config, System) and project.
- **Export** — download audit data as CSV or JSON for compliance review.
- **Real-time events** — subscribe via SSE or event bus to stay informed as events happen.

### API

Access audit data programmatically:

- `GET /api/v1/audit` — query events with filters (category, source, level, container, worktree, time range)
- `GET /api/v1/audit/summary` — aggregate statistics
- `GET /api/v1/audit/export?format=csv|json` — compliance export
- `GET /api/v1/audit/projects` — distinct project names for filtering
- `POST /api/v1/audit` — write custom events (for integrations)
- `DELETE /api/v1/audit` — scoped delete (container, worktree, time range)

## Next steps

- [Custom Images](../devcontainers/) — bring your own image (Dockerfile, devcontainer feature, or fully custom)
- [Integration Paths](../../integration/paths/) — embed Warden in your own application

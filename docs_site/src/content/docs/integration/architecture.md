---
title: Architecture
description: How Warden is structured — layered system, infrastructure layout, and binary variants.
---

## Layered system

Warden is a layered system. Each layer is independently usable:

```
┌───────────────────────────────────┐
│  Frontends (web, TUI)             │  ← Use these directly, or build your own
├───────────────────────────────────┤
│  HTTP API  /api/v1/*              │  ← REST, SSE, WebSocket
├───────────────────────────────────┤
│  Engine (Go library)              │  ← Container lifecycle, security, events
├───────────────────────────────────┤
│  Container image                  │  ← Claude Code + abduco + network isolation
└───────────────────────────────────┘
```

The engine and security model are the core. Everything above is a consumer of the engine's public interfaces — including Warden's own frontends. You can integrate at any layer: use the HTTP API from any language, import the Go client for typed access, or embed the engine directly as a Go library.

## Workspace directory structure

Inside containers, Warden stores agent-specific files and state at dedicated paths:

```
<workspace>/
├── .warden/
│   ├── terminals/           # Terminal state (ephemeral, per-worktree)
│   └── worktrees/           # Non-Claude agent worktrees (Codex, future)
├── .claude/
│   └── worktrees/           # Claude Code worktrees (hardcoded by Claude)
└── ... (project files)
```

- `.warden/terminals/` tracks active terminal processes per worktree. It's ephemeral and reset on container startup.
- `.warden/worktrees/` stores worktrees for non-Claude agents (e.g., Codex). Isolated from Claude's worktrees to prevent conflicts.
- `.claude/worktrees/` is Claude Code's hardcoded location for its own worktrees. Not configurable.

## Infrastructure layout

Warden runs as a host process that manages project containers. Communication flows in three directions: the backend talks to containers via the Docker/Podman API, containers write events to a bind-mounted directory that the backend watches, and the backend fans out state to browsers via SSE and WebSocket.

```
┌──────────────────────────────────────────────────────────────────┐
│  Browser                                                         │
│   ├── REST  /api/v1/*     (project CRUD, settings, audit)        │
│   ├── SSE   /api/v1/events (real-time state, cost, attention)    │
│   └── WS    /api/v1/projects/{id}/ws/{wid}  (terminal I/O)       │
└──────────┬──────────┬──────────┬─────────────────────────────────┘
           │          │          │
┌──────────▼──────────▼──────────▼─────────────────────────────────┐
│  Warden (Go backend, :8090)                                      │
│                                                                  │
│   ┌──────────────┐  ┌──────────────┐  ┌───────────────────────┐  │
│   │ HTTP server  │  │ Event bus    │  │ SQLite DB             │  │
│   │ routes.go    │  │ file watcher │  │ projects, settings,   │  │
│   │ terminal     │  │ fsnotify +   │  │ session_costs, events │  │
│   │ proxy        │  │ polling      │  │                       │  │
│   └──────┬───────┘  └──────▲───────┘  └───────────────────────┘  │
│          │                 │                                     │
│          │ docker exec     │ bind-mounted event directory        │
│          │ (PTY attach,    │ (hook events, cost, heartbeat)      │
│          │  status read)   │                                     │
│          │                 │                                     │
│  ┌───────▼─────────────────┴────────────────────────────────┐    │
│  │  Docker / Podman containers                              │    │
│  │                                                          │    │
│  │  ┌──────────────────┐  ┌──────────────────┐              │    │
│  │  │  project-a       │  │  project-b       │              │    │
│  │  │  abduco          │  │  abduco          │              │    │
│  │  │  hook scripts    │  │  hook scripts    │              │    │
│  │  │  iptables rules  │  │  iptables rules  │              │    │
│  │  └──────────────────┘  └──────────────────┘              │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### Communication pathways

1. **Docker/Podman API** — the backend manages container lifecycle (create, start, stop, remove) and runs exec commands via the container runtime socket. Terminal WebSocket connections are bridged to `abduco -a` sessions inside containers via `docker exec` with TTY mode. Exec is also used to read `.claude.json` for agent status and cost data.

2. **File-based event delivery** — each container has a host directory bind-mounted at `/var/warden/events/`. Claude Code hook scripts (`warden-event.sh`) write atomic JSON files (`.tmp` → rename to `.json`) containing attention state, session lifecycle, tool use, cost updates, and heartbeats. The backend watches all event directories using fsnotify (sub-millisecond on Linux) with a polling fallback every 2 seconds (reliable on all platforms including Docker Desktop). Filesystem permissions handle access control — no network listener or auth token is needed.

3. **SSE + WebSocket** — the event bus fans out state changes to all connected browsers via Server-Sent Events (`worktree_state` for per-worktree attention/terminal changes, `project_state` for aggregated cost + attention per project, `worktree_list_changed`, `budget_exceeded`, `budget_container_stopped`). Terminal I/O streams over WebSocket with binary frames for PTY data and text frames for control messages (resize).

### Single-gateway funnels

Two critical write paths are enforced through single gateways to guarantee invariants:

**Cost writes → `service.PersistSessionCost()`**

All cost data flows through one function regardless of source. This guarantees budget enforcement is never bypassed.

```
Container hook (stop event)  ─┐
warden-capture-cost.sh        ├──► PersistSessionCost() ──► DB write
docker exec fallback read    ─┘         │
                                        ▼
                                  enforceBudget()
                                    ├── warn (SSE broadcast)
                                    ├── stop worktrees
                                    ├── stop container
                                    └── prevent restart (403)
```

**Audit writes → `db.AuditWriter.Write()`**

All audit events flow through the `AuditWriter`, which applies mode filtering before persisting. Direct `db.Store` writes for audit events are prohibited.

```
Container hooks (tool_use, session_start, ...)  ─┐
Backend events (slog warnings/errors)            ├──► AuditWriter.Write()
Frontend events (POST /api/v1/audit)             │         │
Budget enforcement events                       ─┘         ▼
                                                     Mode filter
                                                   (off/standard/detailed)
                                                         │
                                                         ▼
                                                   SQLite events table
```

Standard mode writes only session lifecycle, terminal lifecycle, budget, and system events. Detailed mode adds agent events, tool use, prompts, config, and debug events.

See the [Integration Paths](../paths/) page for the binary variants, key packages, and how to choose an integration approach.

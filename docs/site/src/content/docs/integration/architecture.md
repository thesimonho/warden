---
title: Architecture
description: How Warden is structured — layered system, infrastructure layout, and binary variants.
---

## Layered system

Warden is a three-layer system. Each layer is independently usable and testable:

```
┌─────────────────────────────────────┐
│  Layer 3: Frontends                 │
│  (web dashboard, TUI)               │  ← Use these directly, or build your own
├─────────────────────────────────────┤
│  Layer 2: HTTP API                  │
│  REST + SSE + WebSocket             │  ← /api/v1/* (any language)
├─────────────────────────────────────┤
│  Layer 1: Service                   │
│  Business logic (project/worktree    │  ← Go library (direct import)
│  lifecycle, cost tracking, audit)    │  ← Go client (typed HTTP wrapper)
├─────────────────────────────────────┤
│  Container image                    │  ← Agent CLIs + tmux + isolation
└─────────────────────────────────────┘
```

### How to integrate

The decision tree below shows where to start based on your use case:

```
Are you writing Go?
├─ Yes → Want single-process deployment?
│         ├─ Yes → Layer 1 (Go library): import warden, call warden.New()
│         └─ No  → Layer 2 via Layer 2 client (typed HTTP wrapper)
│
└─ No  → Use Layer 2 from your language (raw HTTP/SSE/WebSocket)
```

**Layer 1 (Service)** is the engine entry point: `warden.New()` returns `*Warden` with `.Service` exposing all operations. The frontends are reference implementations — they use the exact same Layer 2 and Layer 1 interfaces you would.

**Layer 2 (HTTP API)** is REST + SSE + WebSocket at `/api/v1/*`. Works from any language.

**Layer 3 (Frontends)** are the web dashboard (`warden-desktop`) and TUI (`warden-tui`). Both are optional — you can build your own or use the layers directly.

## Project identification

A project is uniquely identified by a **compound primary key**: `(projectID, agentType)`. This allows multiple containers to exist for the same directory, each running a different agent type (e.g., both Claude Code and Codex against the same repo). The `projectID` is a deterministic 12-character hex string derived from the SHA-256 of the resolved absolute host path, while `agentType` is either `"claude-code"` or `"codex"`.

All API routes include the agent type as a path segment: `/api/v1/projects/{projectId}/{agentType}/...`. This ensures operations are scoped to the correct container.

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

Warden runs as a host process that manages project containers. Communication flows in three directions: the backend talks to containers via the Docker API, containers write events to a bind-mounted directory that the backend watches, and the backend fans out state to browsers via SSE and WebSocket.

```
┌──────────────────────────────────────────────────────────────────┐
│  Browser                                                         │
│   ├── REST  /api/v1/*     (project CRUD, settings, audit)        │
│   ├── SSE   /api/v1/events (real-time state, cost, attention)    │
│   └── WS    /api/v1/projects/{id}/{agentType}/ws/{wid}  (terminal I/O) │
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
│  │  Docker containers                                       │    │
│  │                                                          │    │
│  │  ┌──────────────────┐  ┌──────────────────┐              │    │
│  │  │  project-a       │  │  project-b       │              │    │
│  │  │  tmux             │  │  tmux             │              │    │
│  │  │  hook scripts    │  │  hook scripts    │              │    │
│  │  │  iptables rules  │  │  iptables rules  │              │    │
│  │  └──────────────────┘  └──────────────────┘              │    │
│  └──────────────────────────────────────────────────────────┘    │
└──────────────────────────────────────────────────────────────────┘
```

### Communication pathways

1. **Docker API** — the backend manages container lifecycle (create, start, stop, remove) and runs exec commands via the container runtime socket. Terminal WebSocket connections are bridged to `tmux attach-session -t` sessions inside containers via `docker exec` with TTY mode. Exec is also used to read agent config files (e.g., `.claude.json`) for status and cost data.

2. **File-based event delivery** — each container has a host directory bind-mounted at `/var/warden/events/`. Claude Code hook scripts (`warden-event-claude.sh`) write atomic JSON files (`.tmp` → rename to `.json`) containing attention state, session lifecycle, tool use, cost updates, and heartbeats. The backend watches all event directories using fsnotify (sub-millisecond on Linux) with a polling fallback every 2 seconds (reliable on all platforms including Docker Desktop). Filesystem permissions handle access control — no network listener or auth token is needed.

3. **JSONL session parsing** — the primary data source for agent events. Each agent writes JSONL session files to its config directory (`~/.claude/` or `~/.codex/`), which is bind-mounted to the host. The backend watches these locations with `agent.SessionWatcher`, which discovers session files via agent-specific `FindSessionFiles()` methods and tails new lines (polling every 2 seconds). Session discovery is agent-aware: Claude Code scans a per-project directory; Codex reads shell snapshots to filter by project ID. The watcher feeds lines through agent-specific parsers (`agent/claudecode/`, `agent/codex/`) that produce uniform `ParsedEvent` values. These events flow into the event bus for SSE broadcast and audit logging.

   ```
   FindSessionFiles() → SessionWatcher (polling every 2s) → SessionParser.ParseLine() → ParsedEvent → eventbus → SSE
   ```

   JSONL parsing provides session lifecycle, tool use, cost, and prompt events for both agents. Hook-based events (attention/notification state) are supplementary and only available for Claude Code.

4. **SSE + WebSocket** — the event bus fans out state changes to all connected browsers via Server-Sent Events (`worktree_state` for per-worktree attention/terminal changes, `project_state` for aggregated cost + attention per project, `worktree_list_changed`, `budget_exceeded`, `budget_container_stopped`). Terminal I/O streams over WebSocket with binary frames for PTY data and text frames for control messages (resize).

### Single-gateway funnels

Two critical write paths are enforced through single gateways to guarantee invariants:

**Cost writes → `service.PersistSessionCost()`**

All cost data flows through one function regardless of source. This guarantees budget enforcement is never bypassed.

```
JSONL token updates (cost_update) ─┐
docker exec fallback read          ├──► PersistSessionCost() ──► DB write
                                  ─┘         │
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
Container hooks (session_end, attention, ...)    ─┐
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

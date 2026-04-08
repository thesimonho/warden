# Container Environment

## Process Architecture

The container entrypoint starts as root for privileged setup (UID remapping, agent CLI install, runtime installation), then permanently drops to the `warden` user via `exec gosu`. PID 1 runs as `warden` — no root process remains after startup. Network isolation (iptables) is applied separately by the Go server via `docker exec --privileged` after container start.

Each worktree has one process layer in the container:

```
tmux (session manager — holds the PTY alive)
 └── bash
      └── claude/codex (or just bash if the agent exited)
```

| Component | Role                                                   | Can be killed without losing work? |
| --------- | ------------------------------------------------------ | ---------------------------------- |
| **tmux**  | Holds the PTY session alive across viewer disconnects. | No — kills Claude and bash.        |

The browser connects via `GET /api/v1/projects/{projectID}/ws/{wid}` (WebSocket), which the Go backend proxies to `docker exec` with TTY mode. The connection is kept alive with periodic ping/pong heartbeats (30s).

## Env Var Forwarding

The `gosu` exec creates a clean environment, stripping container env vars. The user-phase entrypoint works around this by:

1. Writing all env vars to `/home/warden/.docker_env` at startup (excluding `HOME`, `USER`, `SHELL`, etc.)
2. `.bashrc` sources this file on every new shell

This ensures all vars passed via `docker run -e` are available in terminal sessions.

**Key environment variables set by Warden:**

- `WARDEN_HOST_UID` / `WARDEN_HOST_GID` — host user's UID/GID for UID remapping via `usermod`/`groupmod` (local projects). For remote projects, defaults are used.
- `WARDEN_WORKSPACE_DIR` — container-side workspace path (e.g. `/home/warden/my-project`). Shell scripts use `${WARDEN_WORKSPACE_DIR:-/project}` for backward compatibility.
- `WARDEN_CLONE_URL` — git clone URL for remote projects (optional). When set, entrypoint.sh clones this into the workspace volume instead of bind-mounting.
- `WARDEN_PROJECT_ID` — deterministic 12-char hex identifier (SHA-256 of resolved absolute host path or normalized clone URL). Used by event-posting scripts to tag events with project identity.
- `WARDEN_EVENT_DIR` — bind-mounted event directory path (`/var/warden/events`)
- `WARDEN_AGENT_TYPE` — agent type (`claude-code` or `codex`). Controls which CLI launches in `create-terminal.sh` and which parser/provider the engine uses.
- `WARDEN_NETWORK_MODE` — network isolation mode (`full`/`restricted`/`none`)
- `WARDEN_ALLOWED_DOMAINS` — comma-separated domain list for `restricted` mode (optional)
- `WARDEN_ENABLED_RUNTIMES` — comma-separated list of enabled runtime IDs (e.g. `node,python,go`). Consumed by `install-runtimes.sh` during container startup to install language toolchains.
- `WARDEN_CLAUDE_VERSION` / `WARDEN_CODEX_VERSION` — pinned agent CLI versions from `agent/versions.go`, used by `install-agent.sh`

## Install Marker

The entrypoint writes `/tmp/warden-installs-done` after agent CLI and language runtime installs complete. The Go server (`WaitForInstalls()`) polls this marker before applying network isolation via iptables, ensuring downloads can succeed even when restricted network mode is active. For full network mode containers, this polling is skipped (network isolation doesn't apply).

## Storage

### Terminal Storage

```
/project/.warden/                      # Agent-agnostic Warden directory
  .gitignore                           # Covers entire .warden/ subtree
  terminals/                           # Ephemeral — cleared on container restart
    <worktree-id>/
      exit_code                        # Agent's exit code (present when agent exited)
```

### Worktree Storage

```
/project/.claude/worktrees/          # Claude Code worktrees — persistent, survives container restarts
  <worktree-id>/                       # git worktree checkout (created by Claude's --worktree)
/project/.warden/worktrees/          # Non-Claude agent worktrees — persistent, survives container restarts
  <worktree-id>/                       # git worktree checkout (created by Codex or other agents)
```

## Container-Host Event Contract

Events flow from container to host through two independent channels. Both converge at `eventbus.Store.HandleEvent()`, producing the same `ContainerEvent` type. The `EventSource` field on `ContainerEvent` identifies which channel produced each event (see `eventbus/types.go`).

### Channel 1: Event directory (supplementary, real-time)

Container scripts write JSON files to a bind-mounted directory (`WARDEN_EVENT_DIR=/var/warden/events`). The host watches using fsnotify (fast path) + polling every 2s (reliable fallback). Each file is one `ContainerEvent` JSON object, processed then deleted.

**Contract:**
- Files must be valid JSON matching the `ContainerEvent` schema
- Files must be written atomically (write to temp, rename)
- The `containerName`, `projectId`, `agentType`, and `worktreeId` fields must be set from `WARDEN_*` env vars
- The watcher enforces a safety valve: 50,000 files maximum

**Events from this channel:**
- `terminal_connected` — tmux session created, terminal ready
- `terminal_disconnected` — terminal viewer disconnected, tmux session continues
- `process_killed` — all processes for a worktree terminated
- `session_exit` — agent exited (includes exit code)
- `heartbeat` — periodic liveness signal (every 10s)
- `attention`, `attention_clear`, `needs_answer` — real-time attention state from Claude Code hooks
- Runtime/agent installation progress events

The backend liveness checker monitors heartbeats and marks containers stale after 30s of silence.

### Channel 2: JSONL session files (primary)

Agents write JSONL session files to well-known paths inside the container. The host tails these files via `watcher.FileTailer` with offset tracking (persisted across server restarts). Agent-specific parsers convert raw JSONL lines to `agent.ParsedEvent`, then `service.SessionEventToContainerEvent()` bridges them to `ContainerEvent`.

**Contract:**
- Claude Code writes to `~/.claude/projects/<sanitized-path>/`
- Codex writes to `~/.codex/sessions/YYYY/MM/DD/`
- Files are append-only JSONL (one JSON object per line)
- The host discovers new files by polling every 2s
- JSONL events carry `SourceLine` bytes for content-based audit dedup

**Events from this channel:** session lifecycle, tool use, cost/tokens, user prompts, turn completion, API metrics, context compaction, system info. See `docs/developer/events.md` for the full catalog.

### Channel 3: Backend-generated (synthetic)

The Go backend itself creates `ContainerEvent` values for lifecycle transitions that originate from user actions rather than container scripts (e.g. `ConnectTerminal` and `DisconnectTerminal` in the service layer). These have `Source: SourceBackend`.

### Direction

JSONL is the **primary** data source. Hook events are **supplementary** and transitional — they exist because some data (attention state, terminal lifecycle) is not yet available in agent JSONL. As agents add this data to JSONL, the corresponding hook events can be removed. See `docs/developer/events.md` for the migration status of each event.

## Ports

No dedicated ports for terminals. WebSocket connections are proxied through the backend on the same port as the HTTP API (default 8090 on the host).

## Usage

```bash
# Build
docker build -t claude-project-dev ./container

# Run with project mounted (Warden automatically sets WARDEN_EVENT_DIR)
docker run -d \
  -e ANTHROPIC_API_KEY=sk-xxx \
  -v ./my-project:/project \
  claude-project-dev
```

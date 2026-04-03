# Container Environment

## Process Architecture

The container entrypoint starts as root for privileged setup (UID remapping, iptables), then permanently drops to the `warden` user via `exec gosu`. PID 1 runs as `warden` — no root process remains after startup.

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

- `WARDEN_HOST_UID` / `WARDEN_HOST_GID` — host user's UID/GID for UID remapping via `usermod`/`groupmod`
- `WARDEN_WORKSPACE_DIR` — container-side workspace path (e.g. `/home/warden/my-project`). Shell scripts use `${WARDEN_WORKSPACE_DIR:-/project}` for backward compatibility.
- `WARDEN_PROJECT_ID` — deterministic 12-char hex identifier (SHA-256 of resolved absolute host path). Used by event-posting scripts to tag events with project identity.
- `WARDEN_EVENT_DIR` — bind-mounted event directory path (`/var/warden/events`)
- `WARDEN_AGENT_TYPE` — agent type (`claude-code` or `codex`). Controls which CLI launches in `create-terminal.sh` and which parser/provider the engine uses.
- `WARDEN_NETWORK_MODE` — network isolation mode (`full`/`restricted`/`none`)
- `WARDEN_ALLOWED_DOMAINS` — comma-separated domain list for `restricted` mode (optional)

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

## Event Bus Communication

Terminal lifecycle events and hook events are pushed to the host via file-based delivery. Containers write JSON event files to a bind-mounted directory (`WARDEN_EVENT_DIR=/var/warden/events`). The host watches this directory using fsnotify (fast path) + polling every 2s (reliable fallback):

- `terminal_connected` — tmux session created, terminal ready
- `terminal_disconnected` — terminal viewer disconnected, tmux session continues
- `process_killed` — all processes for a worktree terminated
- `session_exit` — Claude Code exited (includes exit code)
- `heartbeat` — periodic liveness signal (every 10s)

The backend liveness checker monitors heartbeats and marks containers stale after 30s of silence. The watcher enforces a safety valve: 50,000 files maximum.

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

# Architecture

## Mental Model

- A **project** is a workspace directory on the host. It is the user's work — code, git history.
- A **container** is disposable infrastructure that runs an AI coding agent (Claude Code or Codex) against that workspace. Containers can be destroyed and recreated without losing work.
- A **worktree** is an isolated working directory within a project (via `git worktree`). Each worktree is a unit of independent work — a feature, a bugfix, an experiment. Within a worktree, the user can have as many agent conversations as they want, managed entirely by the agent itself.
- For **non-git repos**, there is exactly one implicit worktree — the workspace root. No additional worktrees can be created since there's no git branch isolation. The user still has unlimited conversations within it.

### What Warden manages vs what the agent manages

See the ownership table in [`terminology.md`](terminology.md#what-warden-manages-vs-what-the-agent-manages).

### Terminal Infrastructure

Warden tracks minimal per-worktree terminal state:

```
<workspace>/.warden/terminals/{worktree-id}/
└── exit_code   # Agent exit code (written on exit, stop, or container restart)
```

Where `<workspace>` is the container-side workspace directory (e.g. `/home/warden/<project-name>`, or `/project` for legacy containers).

This directory is ephemeral — stale entries are harmless and reset on container startup. WebSocket connections are managed by the Go backend and do not require port tracking.

## Warden does not manage agent conversations

Each agent CLI has its own internal system for managing conversations (session index, JSONL conversation history, `/resume` for Claude Code). Warden does not duplicate this. Warden manages:

1. **Worktrees** — isolated working directories for parallel work
2. **Terminals** — xterm.js viewers connecting the user to worktree processes via WebSocket
3. **Notifications** — attention state pushed via event bus (Claude Code hooks; Codex pending)
4. **Cost** — parsed from JSONL session files, with agent config as fallback

Conversation management (start, resume, history) is entirely the agent's responsibility.

## Terminal tracking is ephemeral

The `.warden/terminals/` directory only tracks which worktrees have active terminals. It is reset on container startup. Stale entries are harmless. The entrypoint does not need to clean up session state — there is no session state to clean up.

## WebSocket Connections

Terminal connections are proxied through the Go backend via WebSocket at `/api/v1/projects/{projectID}/{agentType}/ws/{wid}`. The backend connects to the container's tmux session via `docker exec` with TTY mode. WebSocket connections are not port-limited — the backend can proxy unlimited concurrent connections.

Each worktree has two attachable tmux sessions and a dedicated WebSocket endpoint for each:

| Endpoint                                                      | tmux session              | Backing script      | Purpose                                     |
| ------------------------------------------------------------- | ------------------------- | ------------------- | ------------------------------------------- |
| `/api/v1/projects/{projectID}/{agentType}/ws/{wid}`           | `warden-{wid}`            | `create-terminal.sh`| Agent session (Claude Code or Codex)        |
| `/api/v1/projects/{projectID}/{agentType}/ws/{wid}/shell`     | `warden-shell-{wid}`      | `create-shell.sh`   | Auxiliary bash shell at the worktree root   |

The auxiliary shell session backs the **Terminal** tab in the webapp's terminal card (alongside Agent and Git Changes) and the TUI `s` keybind. It is created lazily on first attach and persists for the worktree's lifetime — see [`terminology.md#auxiliary-shell-session`](terminology.md#auxiliary-shell-session) for the full ownership and lifecycle rules.

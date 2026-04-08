# Terminology

These terms must be used consistently throughout Warden's codebase, UI text, comments, commit messages, and documentation.

## Core terms

| Term            | Definition                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | Managed by |
| --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------ | ---------- |
| **Project**     | A workspace backed by either a local host directory or a remote git repository. Multiple agent types (claude-code, codex) can run against the same workspace. Each agent type runs in its own container and has independent state (costs, audit logs, worktrees). Identified by a deterministic 12-char hex `project_id` (SHA-256 of resolved host path or normalized clone URL).                                                                                                                                                                                                                                   | Warden     |
| **ProjectID**   | Deterministic 12-character hex identifier computed from SHA-256 of the resolved absolute host path (local projects) or normalized clone URL (remote projects). Part of a compound primary key `(project_id, agent_type)` in the database for associating events/costs/state with a specific agent instance across container rebuilds.                                                                                                                                                                                                                                                                              | Warden     |
| **Project Source** | Whether a project uses a local host directory (`local`) or a remote git clone URL (`remote`). Local projects bind-mount the host directory into the container. Remote projects clone the repository inside the container on first boot. Remote projects can be persistent (workspace stored in a named Docker volume, survives container recreate) or temporary (workspace in the container's writable layer, lost on recreate).                                                                                                                                                                                   | Warden     |
| **Worktree**    | An isolated working directory within a project (via `git worktree`), or the implicit workspace root for non-git repos. The unit of independent work.                                                                                                                                                                                                                                                                                                                                                                                                                                                               | Warden     |
| **Terminal**    | The xterm.js web interface the user sees and types into. A disposable viewer into a worktree. Connects via WebSocket to the Go backend proxy.                                                                                                                                                                                                                                                                                                                                                                                                                                                                      | Warden     |
| **Access Item** | A general-purpose credential and mount provider. Includes built-in items (Git, SSH) for common infrastructure needs and user-defined items for custom access methods. Each item has a detection mechanism to verify availability on the host.                                                                                                                                                                                                                                                                                                                                                                      | Warden     |
| **Agent Type**  | The AI coding agent to run in a container against a directory. Currently `claude-code` or `codex`. Set at project creation time via `WARDEN_AGENT_TYPE` env var. Part of a compound primary key `(project_id, agent_type)` in the database. Changing the agent type for a directory requires creating a new container. Claude Code manages its own worktrees via `--worktree`; Codex worktrees are managed by Warden (`git worktree add`). Codex uses `AGENTS.md` instead of `CLAUDE.md` for project instructions.                                                                                                 | Warden     |
| **Runtime**     | A language runtime (Node.js, Python, Go, Rust, Ruby, Lua) installable in a container. Each runtime is a security declaration: selecting it installs the base runtime AND opens the minimum network surface for its package registry. Node.js is always enabled (required for MCP servers). Other runtimes are auto-detected via marker files (e.g. `go.mod` → Go, `pyproject.toml` → Python). Stored per-project as `enabled_runtimes` in the database. A shared Docker volume (`warden-cache`) persists package caches across container recreates; the volume is only mounted when non-Node runtimes are enabled. | Warden     |

## Banned terms

These terms belong to the agent CLIs and must not be used in Warden's code or UI to avoid confusion:

| Term             | Why it's banned                                                           | Agent meaning                            |
| ---------------- | ------------------------------------------------------------------------- | ---------------------------------------- |
| **Session**      | Both Claude Code and Codex use "session" for a conversation with history. | A single agent conversation (resumable). |
| **Conversation** | Same concept as session in the agents' model.                             | Interchangeable with session.            |

## Process architecture

The container entrypoint starts as root for privileged setup (UID remapping, agent CLI install, runtime installation), then permanently drops to the `warden` user via `exec gosu`. PID 1 runs as `warden` — no root process remains after startup. Network isolation (iptables) is applied separately by the Go server via `docker exec --privileged` after container start.

Each worktree has one process layer in the container. The browser connects to it via WebSocket through the Go backend proxy.

```
tmux (session manager — holds the PTY alive)
 └── bash
      └── claude/codex (or just bash if the agent exited)
```

| Component | Role                                                   | Can be killed without losing work? |
| --------- | ------------------------------------------------------ | ---------------------------------- |
| **tmux**  | Holds the PTY session alive across viewer disconnects. | No — kills the agent and bash.     |

The tmux session is configured with:

- `status off` — no status bar (terminal is rendered in xterm.js)
- `mouse off` — mouse events pass through to xterm.js
- `history-limit 50000` — scrollback buffer for replay on reconnect
- `window-size latest` — resizes to the most recently attached client
- `set-clipboard on` — forwards OSC 52 clipboard sequences from agents to the browser
- `-u` flag — force UTF-8 mode for correct box-drawing character rendering

Agents run with `TMUX` env var unset so they don't detect they're inside tmux.

The browser connects via `GET /api/v1/projects/{projectID}/{agentType}/ws/{wid}` (WebSocket). Before attaching the live stream, the proxy captures the tmux scrollback buffer via `tmux capture-pane` and sends it to the client. This fills the gap between the user's last disconnect and now. The connection is kept alive with periodic ping/pong heartbeats (30s).

## Auto-resume

When a terminal reconnects after the agent has exited (Stop button, container restart, or normal exit), `create-terminal.sh` detects the previous session via `exit_code` file + JSONL session files and launches the agent with `--continue` (Claude Code) or `resume --last` (Codex) instead of starting fresh. The user sees their previous conversation history.

If the resume attempt fails (e.g. no actual conversation to continue despite JSONL files existing from session initialization), the inner script automatically falls back to a fresh session. This prevents the user from being dropped into bare bash when `exit_code` and JSONL files exist but the agent has no conversation to resume.

Auto-resume triggers when:

- **Stop button** — `kill-worktree.sh` writes `exit_code=137` before killing tmux
- **Container restart** — `user-entrypoint.sh` writes `exit_code=137` for orphaned terminal dirs on startup
- **Normal exit** — the inner script writes the actual exit code when the agent exits

Auto-resume does NOT trigger when:

- **Worktree reset** — `ResetWorktree` removes the terminal dir and JSONL session files
- **Worktree deletion** — `RemoveWorktree` removes the entire terminal dir including exit_code

## Terminal actions

| Action         | Verb                                     | What happens                                                                        | Destructive? |
| -------------- | ---------------------------------------- | ----------------------------------------------------------------------------------- | ------------ |
| **Connect**    | `connectTerminal`                        | Start tmux session, launch the agent. Browser connects via WebSocket.               | No           |
| **Disconnect** | `disconnectTerminal`                     | Close viewer. Tmux session keeps running in the background.                         | No           |
| **Reconnect**  | `connectTerminal` (on existing worktree) | Browser reconnects via new WebSocket to existing tmux session.                      | No           |
| **Stop**       | `killWorktreeProcess`                    | Stop agent from running in the background. Writes exit_code for auto-resume.        | Yes          |
| **Reset**      | `resetWorktree`                          | Stop agent, clear session files and terminal state. Audit history preserved.        | Yes          |
| **Delete**     | `removeWorktree`                         | Completely delete worktree from disk (git worktree remove). Removes terminal state. | Yes          |

## Worktree states

| State          | tmux                  | WebSocket | What user sees                          |
| -------------- | --------------------- | --------- | --------------------------------------- |
| **connected**  | Running, agent active | Connected | Green dot, live terminal                |
| **shell**      | Running, agent exited | Connected | Amber dot, bash prompt. Can `--resume`. |
| **background** | Running               | Closed    | Purple dot. Reconnectable.              |
| **stopped**    | Dead                  | N/A       | Gray dot. Click to start fresh.         |

## Agent activity (sub-states of connected)

| Activity            | Meaning                                   | Indicator          |
| ------------------- | ----------------------------------------- | ------------------ |
| **Working**         | Agent is actively generating/executing    | Amber pulsing dot  |
| **Idle**            | Agent is running but not actively working | Muted gray dot     |
| **Need Permission** | Agent needs tool approval                 | Orange pulsing dot |
| **Need Answer**     | Agent is asking a question                | Red pulsing dot    |
| **Need Input**      | Agent is done, waiting for next prompt    | Blue pulsing dot   |

## What Warden manages vs what the agent manages

| Concern       | Owner             | Details                                                                                                                                                                               |
| ------------- | ----------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Worktrees     | Warden (or agent) | Claude Code manages its own worktrees via `--worktree`; Codex worktrees are created by Warden via `git worktree add`. Both use WebSocket connections and heartbeat liveness tracking. |
| Conversations | Agent             | Internal session history, `/resume`, conversation threading — all internal to the agent                                                                                               |
| Cost          | Agent + Warden    | JSONL session files are the primary cost source (parsed by Warden). Claude Code also exposes per-project metrics in `~/.claude.json` as a fallback.                                   |
| Notifications | Warden + Agent    | Claude Code's hook events push attention state via event bus; Warden broadcasts via SSE. Codex does not yet support hooks — attention tracking is a known gap.                        |
| Git branches  | Agent             | The agent manages branches within its worktree. Claude Code manages worktrees natively (`--worktree`); Codex manages branches in Warden-created worktrees.                            |

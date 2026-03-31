# Terminology

These terms must be used consistently throughout Warden's codebase, UI text, comments, commit messages, and documentation.

## Core terms

| Term          | Definition                                                                                                                                           | Managed by |
| ------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------- | ---------- |
| **Project**   | A workspace directory on the host + a container that runs an AI coding agent against it. Identified by a deterministic 12-char hex `project_id` (SHA-256 of resolved host path).                                                                    | Warden     |
| **ProjectID** | Deterministic 12-character hex identifier computed from SHA-256 of the resolved absolute host path. Used as the primary key in the database and for associating events/costs with projects across container rebuilds.     | Warden     |
| **Worktree**  | An isolated working directory within a project (via `git worktree`), or the implicit workspace root for non-git repos. The unit of independent work. | Warden     |
| **Terminal**  | The xterm.js web interface the user sees and types into. A disposable viewer into a worktree. Connects via WebSocket to the Go backend proxy.       | Warden     |
| **Access Item** | A general-purpose credential and mount provider. Includes built-in items (Git, SSH) for common infrastructure needs and user-defined items for custom access methods. Each item has a detection mechanism to verify availability on the host. | Warden |
| **Agent Type** | The AI coding agent to run in a project's container. Currently `claude-code` or `codex`. Set at project creation time via `WARDEN_AGENT_TYPE` env var. Each project is locked to one agent type — changing it requires recreating the container. Claude Code manages its own worktrees via `--worktree`; Codex worktrees are managed by Warden (`git worktree add`). Codex uses `AGENTS.md` instead of `CLAUDE.md` for project instructions. | Warden |

## Banned terms

These terms belong to the agent CLIs and must not be used in Warden's code or UI to avoid confusion:

| Term             | Why it's banned                                                                           | Agent meaning                                |
| ---------------- | ----------------------------------------------------------------------------------------- | -------------------------------------------- |
| **Session**      | Both Claude Code and Codex use "session" for a conversation with history.                 | A single agent conversation (resumable).     |
| **Conversation** | Same concept as session in the agents' model.                                             | Interchangeable with session.                |

## Process architecture

The container entrypoint starts as root for privileged setup (UID remapping, iptables), then permanently drops to the `dev` user via `exec gosu`. PID 1 runs as `dev` — no root process remains after startup.

Each worktree has one process layer in the container. The browser connects to it via WebSocket through the Go backend proxy.

```
abduco (process manager — holds the PTY alive)
 └── bash
      └── claude/codex (or just bash if the agent exited)
```

| Component  | Role                                                    | Can be killed without losing work? |
| ---------- | ------------------------------------------------------- | ---------------------------------- |
| **abduco** | Holds the PTY session alive across viewer disconnects.  | No — kills the agent and bash.     |

The browser connects via `GET /api/v1/projects/{projectID}/ws/{wid}` (WebSocket), which the Go backend proxies to `docker exec` with TTY mode. The connection is kept alive with periodic ping/pong heartbeats (30s).

## Terminal actions

| Action         | Verb                                     | What happens                                                                           | Destructive? |
| -------------- | ---------------------------------------- | -------------------------------------------------------------------------------------- | ------------ |
| **Connect**    | `connectTerminal`                        | Start abduco, launch the agent. Browser connects via WebSocket.                        | No           |
| **Disconnect** | `disconnectTerminal`                     | Close WebSocket. Abduco keeps running.                                                 | No           |
| **Reconnect**  | `connectTerminal` (on existing worktree) | Browser reconnects via new WebSocket to existing abduco session.                       | No           |
| **Kill**       | `killWorktreeProcess`                    | Kill abduco + everything. Process destroyed.                                           | Yes          |

## Worktree states

| State            | abduco                 | WebSocket | What user sees                          |
| ---------------- | ---------------------- | --------- | --------------------------------------- |
| **connected**    | Running, agent active  | Connected | Green dot, live terminal                |
| **shell**        | Running, agent exited  | Connected | Amber dot, bash prompt. Can `--resume`. |
| **background**   | Running                | Closed    | Purple dot. Reconnectable.              |
| **disconnected** | Dead                   | N/A       | Gray dot. Click to start fresh.         |

## Agent activity (sub-states of connected)

| Activity            | Meaning                                        | Indicator          |
| ------------------- | ---------------------------------------------- | ------------------ |
| **Working**         | Agent is actively generating/executing         | Amber pulsing dot  |
| **Idle**            | Agent is running but not actively working      | Muted gray dot     |
| **Need Permission** | Agent needs tool approval                      | Orange pulsing dot |
| **Need Answer**     | Agent is asking a question                     | Red pulsing dot    |
| **Need Input**      | Agent is done, waiting for next prompt         | Blue pulsing dot   |

> **Note:** Attention sub-states (Need Permission, Need Answer, Need Input) currently rely on Claude Code's hook events. Codex does not yet support hooks — attention tracking for Codex is a known upstream gap.

## What Warden manages vs what the agent manages

| Concern       | Owner              | Details                                                                                      |
| ------------- | ------------------ | -------------------------------------------------------------------------------------------- |
| Worktrees     | Warden (or agent)  | Claude Code manages its own worktrees via `--worktree`; Codex worktrees are created by Warden via `git worktree add`. Both use WebSocket connections and heartbeat liveness tracking. |
| Conversations | Agent              | Internal session history, `/resume`, conversation threading — all internal to the agent      |
| Cost          | Agent + Warden     | JSONL session files are the primary cost source (parsed by Warden). Claude Code also exposes per-project metrics in `~/.claude.json` as a fallback. |
| Notifications | Warden + Agent     | Claude Code's hook events push attention state via event bus; Warden broadcasts via SSE. Codex does not yet support hooks — attention tracking is a known gap. |
| Git branches  | Agent              | The agent manages branches within its worktree                                               |

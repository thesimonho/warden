# UX Flows

Complete specification of every user-facing flow in the Warden dashboard, from project creation through worktree and terminal management. This is the source of truth for expected behavior.

## Terminology

All term definitions, banned terms, process architecture, terminal actions, worktree states, and Claude activity sub-states are in [`terminology.md`](terminology.md). Read that file first — the rest of this document uses those terms without re-defining them.

---

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

---

## 1. Add Project

### 1a. Create New Container

**Trigger:** Click "Add Project" > "Create New" tab.

**Form fields:**

- Agent type (required) — Claude Code or OpenAI Codex. Locked after creation.
- Name (required) — container name
- Project directory (required) — host directory to mount as the workspace inside the container
- Image (optional) — defaults to `ghcr.io/thesimonho/warden:latest`
- Skip permissions (optional) — run agent with `--dangerously-skip-permissions` (Claude) or `--dangerously-bypass-approvals-and-sandbox` (Codex)
- Network mode — full, restricted (with domain allowlist), or none
- Cost budget (optional) — per-project spending limit
- Environment variables (optional) — key-value pairs
- Bind mounts — agent config directory (required, auto-added), plus optional additional host directories
- Access items (optional) — credentials to pass through (Git, SSH, custom)

**Expected behavior:**

1. Form validates required fields and absolute paths.
2. Image is pulled if not present locally.
3. Container is created and started with a bind-mounted event directory for host communication.
4. Container appears on the home page in "running" state.
5. If the project already has worktrees from a previous container (in `<workspace>/.claude/worktrees/` for Claude Code or `<workspace>/.warden/worktrees/` for Codex), they appear in the sidebar and are connectable.

**Edge cases:**

- Name already taken: error shown, container not created.
- Image pull fails: error shown.
- Project directory doesn't exist: container starts but project is empty inside.

---

## 2. Home Page

### 2a. Project Grid

**Expected behavior:**

- Each project card shows: name, state, image, OS, active worktree count, total cost, agent status.
- Cards poll for updates (configurable interval, default 10s).
- Running containers show a green state indicator; stopped show grey.
- If any worktree needs user attention, the card shows a notification dot with the highest-priority attention type.

> **On polling vs hooks:** Polling is still needed. The notification hook writes to a file inside the container — someone needs to read it. Container state changes (stop, crash) and cost data still need periodic checking. Polling could be reduced in frequency but not eliminated.

### 2b. Project Actions

| Action  | Trigger                      | Expected Behavior                                                              |
| ------- | ---------------------------- | ------------------------------------------------------------------------------ |
| Open    | Click card                   | Navigate to project detail page                                                |
| Stop    | Stop button                  | Graceful stop (SIGTERM, 30s timeout, then SIGKILL). Card updates to "stopped". |
| Restart | Restart button               | Stop + start. Entrypoint runs, stale terminal state reset.                     |
| Edit    | Edit button (context menu)   | Opens edit dialog (see 3).                                                     |
| Remove  | Remove button (context menu) | Opens remove dialog (see 4).                                                   |

### 2c. Bulk Actions (Select Mode)

**Trigger:** Click any project card to select it, or click empty space to deselect all.

**Expected behavior:**

- Cards become selectable by clicking (ring outline indicates selection).
- Bottom toolbar shows: selected count, Start, Stop, Restart, Clear.

---

## 3. Edit Project

**Trigger:** Edit button on a project card.

**Expected behavior:**

1. Dialog opens pre-populated with current container config (agent type, name, image, project directory, env vars, skip permissions).
2. Agent type, name, and project directory are read-only (cannot change after creation).
3. On save: old container is stopped and removed, new one is created with updated config.
4. The new container starts in stopped state — user must manually start it.
5. All data is preserved — worktrees on the bind mount, agent conversation history in config directories (`~/.claude/`, `~/.codex/`).

**Edge cases:**

- Container is running: still recreated (stop + remove + create).
- Image pull fails during recreation: error shown, old container is already gone.

---

## 4. Remove Project

**Trigger:** Remove button on a project card.

**Expected behavior:**

1. Confirmation dialog opens.
2. Checkbox option: "Also delete container" (stop + remove the Docker container).
3. If checkbox is unchecked: project is removed from the dashboard config only. Container continues to exist.
4. If checkbox is checked: container is deleted first (may take up to 30s), then project is removed from config.
5. Dialog shows loading state (spinner, disabled buttons) while operation runs.
6. Dialog cannot be closed while operation is in progress.
7. Worktrees and agent data are NOT deleted — they belong to the project, not the container.
8. If container deletion fails, a user-visible error notification is shown. The project is still removed from the dashboard config.

**Edge cases:**

- Container already gone (not found): checkbox is disabled. Only removes from config.

---

## 5. Project Detail Page

**Trigger:** Click a project card on the home page.

**Layout:** Worktree sidebar (left) + terminal area (right).

**Expected behavior:**

- Only shown when container state is "running".
- When container is not running: show "Container is not running" message. The user can start the container from the project card actions on the home page.

---

## 6. Worktrees

### Core Principle

A worktree is a unit of independent work. The user creates worktrees to have the agent work on separate things in parallel, each on its own branch. Within a worktree, the agent manages conversations internally — Warden does not track or manage conversations.

### Worktree States

| State          | Meaning                                                    | What happens on click                                   |
| -------------- | ---------------------------------------------------------- | ------------------------------------------------------- |
| **connected**  | Terminal running, agent active                             | Show the existing terminal                              |
| **shell**      | Agent exited, bash shell still alive in tmux               | Show the terminal (with "Agent exited" indicator)       |
| **background** | tmux session alive, WebSocket closed (viewer disconnected) | Reconnect: reconnect WebSocket to existing tmux session |
| **stopped**    | No processes running                                       | Connect: start new terminal, launch agent               |

### 6a. Create Worktree

**Trigger:** Click "New Worktree" button in sidebar.

**For git repos:**

1. Dialog opens with optional name field.
2. On create: for Claude Code, `claude --worktree <id>` creates the checkout at `<workspace>/.claude/worktrees/{id}/`. For Codex, Warden runs `git worktree add` to create the checkout at `<workspace>/.warden/worktrees/{id}/`, then launches `codex --no-alt-screen` in the worktree directory.
3. tmux session started, agent launched in the worktree.
4. If skip permissions enabled, the agent runs with `--dangerously-skip-permissions` (Claude) or `--dangerously-bypass-approvals-and-sandbox` (Codex).
5. Worktree appears in sidebar as "connected" with a green dot.
6. Terminal loads and connects via WebSocket.

**For non-git repos:**

- There is exactly one implicit worktree: the workspace root.
- The "New Worktree" button is hidden or disabled.
- The single worktree is always present in the sidebar.
- Clicking it connects a terminal running the agent without `--worktree`.

**Edge cases:**

- Container stops during creation: error shown.

### 6b. Click Worktree (Connected)

**Trigger:** Click a connected worktree in the sidebar.

**Expected behavior:**

1. Terminal shows the active tmux session via xterm.js.
2. Green "Connected" dot shown.
3. Switching between worktrees preserves terminal state (terminals stay alive, hidden via CSS).
4. If Claude needs attention, notification dot shown on the worktree card.

### 6c. Click Worktree (Shell — Agent exited, terminal alive)

**Trigger:** Click a worktree where the agent exited but the tmux bash shell is still running.

**Expected behavior:**

1. Terminal shows the bash shell via reconnected WebSocket.
2. Indicator shows "Agent exited" with the exit status.
3. User can type commands in the shell, run `claude --resume` (or start a new Codex session), or start a fresh agent conversation.
4. The worktree card shows an amber/yellow dot to distinguish from fully connected.

### 6d. Click Worktree (Background — tmux session alive, WebSocket closed)

**Trigger:** Click a worktree where the WebSocket was closed but the tmux session is still running.

**Expected behavior:**

1. A new WebSocket connection is established to the existing tmux session.
2. The user sees the terminal resume where it was — same Claude process, same output.
3. Worktree state becomes "connected" or "shell" depending on whether Claude is still running.

### 6e. Click Worktree (Disconnected)

**Trigger:** Click a worktree with no processes running.

**Expected behavior:**

1. Automatically starts a new terminal — no intermediate screen or button.
2. Starts tmux session with the agent launched in the worktree directory (`--worktree <id>` for Claude Code git repos, or in the worktree path for Codex).
3. This is a fresh agent conversation. The user can `/resume` a previous conversation if they want — that's the agent's UX, not Warden's.
4. Worktree state becomes "connected", terminal loads via WebSocket.

### 6f. Disconnect Terminal

**Trigger:** Disconnect button on a terminal panel, or closing a canvas panel.

**Expected behavior:**

1. The WebSocket connection is closed.
2. The tmux session continues running in the background. The agent (if running) is unaffected.
3. Worktree transitions to "background" state.
4. The worktree remains in the sidebar — clicking it reconnects (see 6d).
5. No confirmation dialog needed — this is a non-destructive action.

### 6g. Stop Worktree

**Trigger:** "Stop" action in the sidebar context menu. Stops the agent from running in the background.

**Expected behavior:**

1. `exit_code=137` is written for auto-resume on next connect.
2. The tmux session is killed. The agent receives SIGHUP/SIGTERM.
3. Stale tracking files are cleaned up (exit_code is preserved).
4. Worktree transitions to "stopped" state.
5. The worktree card shows a grey dot.
6. The worktree remains in the sidebar — reconnecting triggers auto-resume.

### 6h. Delete Worktree

**Trigger:** "Delete" action in the sidebar context menu. Completely removes the worktree from disk.

**Expected behavior:**

1. The tmux session is killed (if running).
2. The git worktree is removed (`git worktree remove --force`).
3. The entire terminal tracking directory is removed (including exit_code — no auto-resume).
4. The worktree is removed from the sidebar.

### 6i. Worktree Lifecycle Across Container Events

| Container Event | Impact on Terminals                                     | Impact on Worktrees                                                                                  |
| --------------- | ------------------------------------------------------- | ---------------------------------------------------------------------------------------------------- |
| Stop            | All tmux sessions killed. WebSocket connections closed. | Git worktrees preserved on bind mount. All become "stopped". Auto-resume on reconnect.               |
| Start / Restart | Entrypoint writes exit_code for orphaned terminals.     | Worktrees rediscovered from filesystem. All start as "stopped". Auto-resume on reconnect.            |
| Recreate (edit) | Old container removed.                                  | Worktrees preserved on bind mount. Reconnectable in new container.                                   |
| Delete          | Container removed.                                      | Worktrees preserved on bind mount. Available when a new container mounts the same project directory. |

### 6j. Worktree Attention/Notifications

Worktrees can require user attention. Attention state is pushed via the event bus:

1. For Claude Code: hook events (Notification, PreToolUse, UserPromptSubmit) are written by `warden-event-claude.sh` to the bind-mounted event directory. The backend's file watcher detects them, tracks attention state per worktree, and broadcasts changes via SSE. Codex does not yet support hooks — attention tracking for Codex projects is a known upstream gap.

| Attention Type       | Priority    | Indicator                                           |
| -------------------- | ----------- | --------------------------------------------------- |
| `permission_prompt`  | 3 (highest) | Orange dot — Claude needs tool approval             |
| `elicitation_dialog` | 2           | Red dot — Claude is asking a question               |
| `idle_prompt`        | 1           | Blue dot — Claude finished, waiting for next prompt |

Project cards on the home page show the highest-priority attention type across all worktrees.

### 6k. Cost Tracking

Cost data comes from two sources:

1. **JSONL session files** (primary) — the Go backend parses session JSONL files to extract token usage, compute cost, and track per-session spending. This is the primary data source for both Claude Code and Codex.
2. **Agent config files** (fallback) — Claude Code exposes per-project metrics in `~/.claude.json`. Used as a fallback when JSONL data is not available.

For Claude Code API users, cost is the actual API spend. For Claude Pro/Max subscribers, cost reflects real usage but is not billed directly. For Codex, cost is estimated from token counts using published pricing.

For projects with multiple worktrees, cost is the sum across all worktree sessions.

---

## 7. Settings

**Trigger:** Gear icon on home page.

**Settings:**

- Notification poll interval.
- Browser notifications toggle.

**Edge cases:**

- Docker socket inactive: hint shown to start the Docker daemon.

---

## Architectural Decisions

### Warden does not manage agent conversations

Each agent CLI has its own internal system for managing conversations (session index, JSONL conversation history, `/resume` for Claude Code). Warden does not duplicate this. Warden manages:

1. **Worktrees** — isolated working directories for parallel work
2. **Terminals** — xterm.js viewers connecting the user to worktree processes via WebSocket
3. **Notifications** — attention state pushed via event bus (Claude Code hooks; Codex pending)
4. **Cost** — parsed from JSONL session files, with agent config as fallback

Conversation management (start, resume, history) is entirely the agent's responsibility.

### Terminal tracking is ephemeral

The `.warden/terminals/` directory only tracks which worktrees have active terminals. It is reset on container startup. Stale entries are harmless. The entrypoint does not need to clean up session state — there is no session state to clean up.

### WebSocket Connections

Terminal connections are proxied through the Go backend via WebSocket at `/api/projects/{id}/ws/{wid}`. The backend connects to the container's tmux session via `docker exec` with TTY mode. WebSocket connections are not port-limited — the backend can proxy unlimited concurrent connections.

### Worktree discovery

Worktrees are discovered by listing the filesystem:

- **Git repos:** `git worktree list` inside the container gives all worktrees with their paths and branches.
- **Non-git repos:** Exactly one implicit worktree — the workspace root itself.

This is always consistent because git is the source of truth for worktrees, just as the agent is the source of truth for conversations.

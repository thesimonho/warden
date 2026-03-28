# UX Flows

Complete specification of every user-facing flow in the Warden dashboard, from project creation through worktree and terminal management. This is the source of truth for expected behavior.

## Terminology

All term definitions, banned terms, process architecture, terminal actions, worktree states, and Claude activity sub-states are in [`terminology.md`](terminology.md). Read that file first — the rest of this document uses those terms without re-defining them.

---

## Mental Model

- A **project** is a workspace directory on the host. It is the user's work — code, git history.
- A **container** is disposable infrastructure that runs Claude Code against that workspace. Containers can be destroyed and recreated without losing work.
- A **worktree** is an isolated working directory within a project (via `git worktree`). Each worktree is a unit of independent work — a feature, a bugfix, an experiment. Within a worktree, the user can have as many Claude Code conversations as they want, managed entirely by Claude Code itself.
- For **non-git repos**, there is exactly one implicit worktree — the workspace root. No additional worktrees can be created since there's no git branch isolation. The user still has unlimited conversations within it.

### What Warden manages vs what Claude Code manages

See the ownership table in [`terminology.md`](terminology.md#what-warden-manages-vs-what-claude-code-manages).

### Terminal Infrastructure

Warden tracks minimal per-worktree terminal state:

```
<workspace>/.warden-terminals/{worktree-id}/
└── exit_code   # Claude's exit code (written when Claude exits)
```

Where `<workspace>` is the container-side workspace directory (e.g. `/home/dev/<project-name>`, or `/project` for legacy containers).

This directory is ephemeral — stale entries are harmless and reset on container startup. WebSocket connections are managed by the Go backend and do not require port tracking.

---

## 1. Add Project

### 1a. Create New Container

**Trigger:** Click "Add Project" > "Create New" tab.

**Form fields:**

- Name (required) — container name
- Project directory (required) — host directory to mount as the workspace inside the container
- Image (optional) — defaults to `ghcr.io/thesimonho/warden:latest`
- Claude config path (optional) — host `~/.claude` directory to mount
- Environment variables (optional) — key-value pairs
- Skip permissions (optional) — run Claude with `--dangerously-skip-permissions`

**Expected behavior:**

1. Form validates required fields and absolute paths.
2. Image is pulled if not present locally.
3. Container is created and started with a bind-mounted event directory for host communication.
4. Container appears on the home page in "running" state.
5. If the project already has worktrees from a previous container (in `<workspace>/.claude/worktrees/`), they appear in the sidebar and are connectable.

**Edge cases:**

- Name already taken: error shown, container not created.
- Image pull fails: error shown.
- Project directory doesn't exist: container starts but project is empty inside.



---

## 2. Home Page

### 2a. Project Grid

**Expected behavior:**

- Each project card shows: name, state, image, OS, active worktree count, total cost, Claude status.
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

**Trigger:** Click "Select" button.

**Expected behavior:**

- Cards become selectable with checkboxes.
- Bottom toolbar shows: selected count, Open in Workspace, Stop, Restart, Cancel.
- Open in Workspace navigates to a multi-project workspace view.

---

## 3. Edit Project

**Trigger:** Edit button on a project card.

**Expected behavior:**

1. Dialog opens pre-populated with current container config (name, image, project directory, env vars, claude config, skip permissions).
2. Name and project directory are read-only (cannot change after creation).
3. On save: old container is stopped and removed, new one is created with updated config.
4. The new container starts in stopped state — user must manually start it.
5. All data is preserved — worktrees on the bind mount, Claude Code's conversation history in `~/.claude/`.

**Edge cases:**

- Container is running: still recreated (stop + remove + create).
- Image pull fails during recreation: error shown, old container is already gone.

---

## 4. Remove Project

**Trigger:** Remove button on a project card.

**Expected behavior:**

1. Confirmation dialog opens.
2. Checkbox option: "Also delete container" (stop + remove the Docker/Podman container).
3. If checkbox is unchecked: project is removed from the dashboard config only. Container continues to exist.
4. If checkbox is checked: container is deleted first (may take up to 30s), then project is removed from config.
5. Dialog shows loading state (spinner, disabled buttons) while operation runs.
6. Dialog cannot be closed while operation is in progress.
7. Worktrees and Claude Code data are NOT deleted — they belong to the project, not the container.
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

A worktree is a unit of independent work. The user creates worktrees to have Claude work on separate things in parallel, each on its own branch. Within a worktree, Claude Code manages conversations internally — Warden does not track or manage conversations.

### Worktree States

| State            | Meaning                                        | What happens on click                                 |
| ---------------- | ---------------------------------------------- | ----------------------------------------------------- |
| **connected**    | Terminal running, Claude active                | Show the existing terminal                            |
| **shell**        | Claude exited, bash shell still alive in abduco | Show the terminal (with "Claude exited" indicator)    |
| **background**   | abduco alive, WebSocket closed (viewer disconnected) | Reconnect: reconnect WebSocket to existing abduco |
| **disconnected** | No processes running                           | Connect: start new terminal, launch Claude Code       |

### 6a. Create Worktree

**Trigger:** Click "New Worktree" button in sidebar.

**For git repos:**

1. Dialog opens with optional name field.
2. On create: `git worktree add` creates an isolated checkout at `<workspace>/.claude/worktrees/{id}/`.
3. abduco session started, Claude Code launched with `--worktree <id>`.
4. If skip permissions enabled, Claude runs with `--dangerously-skip-permissions`.
5. Worktree appears in sidebar as "connected" with a green dot.
6. Terminal loads and connects via WebSocket.

**For non-git repos:**

- There is exactly one implicit worktree: the workspace root.
- The "New Worktree" button is hidden or disabled.
- The single worktree is always present in the sidebar.
- Clicking it connects a terminal running Claude Code without `--worktree`.

**Edge cases:**

- Container stops during creation: error shown.

### 6b. Click Worktree (Connected)

**Trigger:** Click a connected worktree in the sidebar.

**Expected behavior:**

1. Terminal shows the active abduco session via xterm.js.
2. Green "Connected" dot shown.
3. Switching between worktrees preserves terminal state (terminals stay alive, hidden via CSS).
4. If Claude needs attention, notification dot shown on the worktree card.

### 6c. Click Worktree (Shell — Claude exited, terminal alive)

**Trigger:** Click a worktree where Claude exited but the abduco bash shell is still running.

**Expected behavior:**

1. Terminal shows the bash shell via reconnected WebSocket.
2. Indicator shows "Claude exited" with the exit status.
3. User can type commands in the shell, run `claude --resume`, or start a fresh Claude conversation.
4. The worktree card shows an amber/yellow dot to distinguish from fully connected.

### 6d. Click Worktree (Background — abduco alive, WebSocket closed)

**Trigger:** Click a worktree where the WebSocket was closed but abduco is still running.

**Expected behavior:**

1. A new WebSocket connection is established to the existing abduco session.
2. The user sees the terminal resume where it was — same Claude process, same output.
3. Worktree state becomes "connected" or "shell" depending on whether Claude is still running.

### 6e. Click Worktree (Disconnected)

**Trigger:** Click a worktree with no processes running.

**Expected behavior:**

1. Automatically starts a new terminal — no intermediate screen or button.
2. Starts abduco with Claude Code launched in the worktree directory (`--worktree <id>` for git repos).
3. This is a fresh Claude Code conversation. The user can `/resume` a previous conversation if they want — that's Claude Code's UX, not Warden's.
4. Worktree state becomes "connected", terminal loads via WebSocket.

### 6f. Disconnect Terminal

**Trigger:** Disconnect button on a terminal panel, or closing a canvas panel.

**Expected behavior:**

1. The WebSocket connection is closed.
2. The abduco session continues running in the background. Claude Code (if running) is unaffected.
3. Worktree transitions to "background" state.
4. The worktree remains in the sidebar — clicking it reconnects (see 6d).
5. No confirmation dialog needed — this is a non-destructive action.

### 6g. Kill Worktree Process

**Trigger:** Explicit "kill" action in the sidebar (e.g. context menu). This is a rare, intentional action.

**Expected behavior:**

1. The abduco process is killed. Claude Code receives SIGHUP/SIGTERM.
2. Terminal tracking directory is removed.
3. Worktree transitions to "disconnected" state.
4. The worktree card shows a grey dot.
5. The worktree remains in the sidebar — always reconnectable (starts fresh).

There is no "delete worktree" action through Warden's UI. Worktrees persist as long as the git repo has them. Cleanup is a git operation (`git worktree remove`), not a Warden operation.

### 6h. Worktree Lifecycle Across Container Events

| Container Event | Impact on Terminals                             | Impact on Worktrees                                                                          |
| --------------- | ----------------------------------------------- | -------------------------------------------------------------------------------------------- |
| Stop            | All abduco killed. WebSocket connections closed. | Git worktrees preserved on bind mount. All become "disconnected".                            |
| Start / Restart | Entrypoint runs. Terminal tracking state reset. | Worktrees rediscovered from filesystem. All start as "disconnected".                         |
| Recreate (edit) | Old container removed.                          | Worktrees preserved on bind mount. Reconnectable in new container.                           |
| Delete          | Container removed.                              | Worktrees preserved on bind mount. Available when a new container mounts the same project directory. |

### 6i. Worktree Attention/Notifications

Worktrees can require user attention. Attention state is pushed via the event bus:

1. Claude Code's hook events (Notification, PreToolUse, UserPromptSubmit) are written by `warden-event.sh` to the bind-mounted event directory. The backend's file watcher detects them, tracks attention state per worktree, and broadcasts changes via SSE.

| Attention Type       | Priority    | Indicator                                           |
| -------------------- | ----------- | --------------------------------------------------- |
| `permission_prompt`  | 3 (highest) | Orange dot — Claude needs tool approval             |
| `elicitation_dialog` | 2           | Red dot — Claude is asking a question               |
| `idle_prompt`        | 1           | Blue dot — Claude finished, waiting for next prompt |

Project cards on the home page show the highest-priority attention type across all worktrees.

### 6j. Cost Tracking

Cost data comes from Claude Code's native metrics in `~/.claude.json`, read by Warden's agent status provider. This gives per-project cost data (tokens, cost, duration, model) keyed by working directory path. No JSONL parsing needed.

For projects with multiple worktrees, cost is the sum across all worktree paths that Claude Code has tracked.

---

## 7. Settings

**Trigger:** Gear icon on home page.

**Settings:**

- Runtime selection (Docker/Podman) — shows available runtimes with socket status.
- Notification poll interval.
- Browser notifications toggle.

**Edge cases:**

- Selected runtime becomes unavailable: auto-falls back to first available runtime.
- Runtime change requires dashboard restart (banner shown).
- Podman socket inactive: hint shown to enable via `systemctl --user enable --now podman.socket`.

---

## Architectural Decisions

### Warden does not manage Claude Code conversations

Claude Code has a complete internal system for managing conversations (session index, JSONL conversation history, `/resume`). Warden does not duplicate this. Warden manages:

1. **Worktrees** — isolated working directories for parallel work
2. **Terminals** — xterm.js viewers connecting the user to worktree processes via WebSocket
3. **Notifications** — attention state pushed via event bus from Claude Code hooks
4. **Cost** — read from Claude Code's native per-project metrics

Conversation management (start, resume, history) is entirely Claude Code's responsibility.

### Terminal tracking is ephemeral

The `.warden-terminals/` directory only tracks which worktrees have active terminals. It is reset on container startup. Stale entries are harmless. The entrypoint does not need to clean up session state — there is no session state to clean up.

### WebSocket Connections

Terminal connections are proxied through the Go backend via WebSocket at `/api/projects/{id}/ws/{wid}`. The backend connects to the container's abduco session via `docker exec` with TTY mode. WebSocket connections are not port-limited — the backend can proxy unlimited concurrent connections.

### Worktree discovery

Worktrees are discovered by listing the filesystem:

- **Git repos:** `git worktree list` inside the container gives all worktrees with their paths and branches.
- **Non-git repos:** Exactly one implicit worktree — the workspace root itself.

This is always consistent because git is the source of truth for worktrees, just as Claude Code is the source of truth for conversations.

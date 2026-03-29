# Container Scripts

All scripts live at `container/scripts/` and are copied to `/usr/local/bin/` inside the container.

## Terminal Lifecycle

### create-terminal.sh

Accepts `<worktree-id> [--skip-permissions]`:

- Validates worktree ID (alphanumeric, hyphens, underscores, dots; no path traversal)
- Starts abduco session `warden-<worktree-id>`
- Launches Claude Code with `--worktree <worktree-id>` (no `--session-id`)
- If `--skip-permissions` is passed, adds `--dangerously-skip-permissions` to the Claude invocation
- When Claude exits, captures cost from `.claude.json` via `warden-capture-cost.sh`, records exit code to `.warden-terminals/<worktree-id>/exit_code`, pushes `session_exit` event, then drops to `exec bash` so the shell stays alive
- Outputs JSON `{"worktreeId":"..."}` to stdout

### disconnect-terminal.sh

- Kills abduco session via `pkill`
- Pushes `terminal_disconnected` event (via `warden-push-event.sh`)
- Removes the terminal tracking directory entry (NOT the git worktree itself)
- Outputs `{"status":"disconnected"}` to stdout

### kill-worktree.sh

- Kills abduco for a worktree
- Pushes `process_killed` event (via `warden-push-event.sh`)
- Removes all terminal tracking state

## Event Scripts

### warden-heartbeat.sh

- Runs as background process (started by entrypoint.sh)
- Writes a heartbeat event to the bind-mounted event directory every 10s
- Allows backend liveness checker to detect stale containers

### warden-push-event.sh

Thin wrapper around `warden-write-event.sh` for terminal lifecycle events (`terminal_disconnected`, `process_killed`). Used by `disconnect-terminal.sh` and `kill-worktree.sh`.

### warden-write-event.sh

- Shared helper script used by all event-posting scripts (via `warden-push-event.sh` and `warden-event.sh`)
- Atomically writes events to the bind-mounted event directory (write to `.tmp`, rename to `.json`)
- Filename format: `<epoch_ns>-<pid>.json`

### setup-network-isolation.sh

Configures iptables OUTPUT rules based on `WARDEN_NETWORK_MODE`. Runs in the entrypoint before user code executes. See [security.md](security.md) for the full network isolation details.

### warden-event.sh

Event bus dispatcher: writes hook events to bind-mounted event directory via `warden-write-event.sh`. The dispatcher determines the worktree ID from Claude's cwd path (`/project/.claude/worktrees/<id>` → id, `/project` → "main").

### warden-cost-lib.sh

Shared cost functions: `read_cost_data` + `send_cost_event` (sourced by `warden-event.sh` and `warden-capture-cost.sh`).

### warden-capture-cost.sh

Post-exit cost capture: sources `warden-cost-lib.sh` and fires stop event via `warden-write-event.sh`.

## Attention Tracking

Claude Code's `Notification` hook (configured via managed settings at `/etc/claude-code/managed-settings.json`) pushes the notification type to the event bus via `warden-event.sh`. Attention types:

- `permission_prompt` — Claude needs tool approval
- `idle_prompt` — Claude is done and waiting for the next prompt
- `elicitation_dialog` — Claude is asking the user a question
- `auth_success` — authentication completed (not treated as attention-requiring)

`UserPromptSubmit` and `PreToolUse` hooks push attention-clear events when the user responds or Claude resumes work. `PreToolUse` with `tool_name == "AskUserQuestion"` pushes a `needs_answer` event instead. All hooks merge with user/project hooks — they never override user configuration.

## Audit Logging Modes

The `auditLogMode` setting (off/standard/detailed) controls which Claude Code hook events are captured by managed settings and written to the event directory. The backend broadcasts mode changes to all running containers via SSE.

**off mode** — No hooks registered. No events logged.

**standard mode** — Only attention-tracking hooks registered (`Notification`, `UserPromptSubmit`, `PreToolUse`). Terminal lifecycle and cost events are always logged.

**detailed mode** — All major Claude Code hooks registered, including: `SessionStart`, `SessionEnd`, `Stop`, `Notification`, `UserPromptSubmit`, `PreToolUse`, `PostToolUseFailure`, `StopFailure`, `PermissionRequest`, `SubagentStart`/`SubagentStop`, `ConfigChange`, `InstructionsLoaded`, `TaskCompleted`, `Elicitation`/`ElicitationResult`.

**Not hooked**: `WorktreeCreate` — registering any hook replaces Claude Code's default `git worktree` creation behavior, breaking worktree setup.

# Container Scripts

Scripts are organized into subdirectories under `container/scripts/` and copied to `/usr/local/bin/` inside the container by `install-warden.sh`.

```
container/scripts/
  install-tools.sh              # Wrapper for devcontainer path (calls sub-scripts)
  install-system-deps.sh        # System packages, GitHub CLI, Node.js, abduco/gosu, bubblewrap (devcontainer)
  install-user.sh               # warden user, workspace dirs, .profile env forwarding
  install-claude.sh             # Claude Code CLI + managed-settings.json hooks
  install-codex.sh              # Codex CLI (npm install -g @openai/codex)
  install-warden.sh             # Copy scripts to /usr/local/bin/, create /project
  shared/                       # Agent-agnostic scripts
    entrypoint.sh               # Root-phase: UID remapping, iptables, exec gosu
    user-entrypoint.sh          # User-phase (PID 1): env forwarding, git config, heartbeat
    create-terminal.sh          # Agent-aware terminal creation (branches on WARDEN_AGENT_TYPE)
    disconnect-terminal.sh      # Disconnect viewer, abduco continues
    kill-worktree.sh            # Kill abduco + all processes for a worktree
    warden-write-event.sh       # Shared atomic event file write library
    warden-push-event.sh        # Terminal lifecycle event helper
    warden-heartbeat.sh         # Background heartbeat (every 10s)
    setup-network-isolation.sh  # iptables OUTPUT rules for network modes
  claude/
    warden-event-claude.sh      # Claude attention state dispatcher (notification, pre_tool_use, user_prompt)
  codex/
    warden-event-codex.sh       # Placeholder (Codex has no hooks yet)
```

## Install Scripts

The Dockerfile calls each install script as a separate `RUN` instruction for layer caching. The devcontainer feature path calls `install-tools.sh` which orchestrates all sub-scripts in order.

| Script | Layer | Changes when... |
|--------|-------|-----------------|
| `install-system-deps.sh` | 1 | New system packages added |
| `install-user.sh` | 2 | User setup or env forwarding changes |
| `install-claude.sh` | 3 | Claude CLI releases or hook config changes |
| `install-codex.sh` | 4 | Codex CLI releases |
| `install-warden.sh` | 5 | Any Warden script changes (most frequent) |

## Terminal Lifecycle

### create-terminal.sh

Accepts `<worktree-id> [--skip-permissions]`. Branches on `WARDEN_AGENT_TYPE` env var:

- **claude-code** (default): launches `claude --worktree <id>` in `.claude/worktrees/<id>/` (Claude manages worktrees natively). Adds `--dangerously-skip-permissions` if requested.
- **codex**: creates the git worktree manually (`git worktree add`) in `.warden/worktrees/<id>/` if it doesn't exist, with fallback to use existing branch if worktree creation fails, then launches `codex --no-alt-screen` in the worktree directory (the `--no-alt-screen` flag disables alternate screen mode so terminal scrollback is preserved). Adds `--dangerously-bypass-approvals-and-sandbox` if skip-permissions is requested.

When the agent exits: records exit code, pushes `session_exit` event, drops to `exec bash`.

### disconnect-terminal.sh

Pushes `terminal_disconnected` event. Removes terminal tracking state. Abduco continues running.

### kill-worktree.sh

Kills abduco for a worktree. Pushes `process_killed` event. Removes all terminal tracking state.

## Event Scripts

### warden-write-event.sh

Shared library sourced by all event-producing scripts. Provides:

- `warden_extract_field "$json" "field"` — bash regex extraction for simple string values (no jq fork)
- `warden_build_event_json "$type" "$data"` — constructs event envelope JSON (requires `CONTAINER_NAME`, `PROJECT_ID`, `WORKTREE_ID`)
- `warden_write_event "$json"` — atomic write to event directory (`.tmp` → `.json` rename). Extracts worktree ID from `.claude/worktrees/` and `.warden/worktrees/` paths.

### warden-heartbeat.sh

Background process (started by entrypoint.sh). Writes heartbeat event every 10s for backend liveness detection.

### warden-push-event.sh

Thin wrapper for terminal lifecycle events (`terminal_disconnected`, `process_killed`, `session_exit`).

### claude/warden-event-claude.sh

Attention state dispatcher for Claude Code hooks. With the JSONL session parser as primary data source, only three hooks remain active:

- **Notification** → `attention` event (permission prompts, idle, elicitation)
- **PreToolUse** → `attention_clear` or `needs_answer` (for AskUserQuestion)
- **UserPromptSubmit** → `attention_clear` + `user_prompt` audit event

All other events (session lifecycle, tool use, cost, etc.) are parsed from the JSONL session file by the Go backend.

### codex/warden-event-codex.sh

Placeholder. Codex does not currently support hooks (upstream gap). When hook support is added, this script will handle attention state events.

## Attention Tracking

Claude Code's hooks push attention state to the event bus via `claude/warden-event-claude.sh`. Attention types from the Notification hook:

- `permission_prompt` — Claude needs tool approval
- `idle_prompt` — Claude is done, waiting for the next prompt
- `elicitation_dialog` — Claude is asking the user a question

`UserPromptSubmit` and `PreToolUse` hooks push attention-clear events when the user responds. `PreToolUse` with `tool_name == "AskUserQuestion"` pushes `needs_answer` instead.

Codex attention tracking is a known upstream gap — no hooks available yet.

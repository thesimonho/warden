# Container Scripts

Scripts are organized into subdirectories under `container/scripts/` and copied to `/usr/local/bin/` inside the container by `install-warden.sh`.

```
container/scripts/
  install-tools.sh              # Wrapper for devcontainer path (calls sub-scripts)
  install-system-deps.sh        # System packages, GitHub CLI, Node.js, tmux/gosu, bubblewrap (devcontainer)
  install-user.sh               # warden user, workspace dirs, .profile env forwarding
  install-warden.sh             # Copy scripts to /usr/local/bin/, create /project
  shared/                       # Agent-agnostic scripts
    entrypoint.sh               # Root-phase: UID remapping, git clone (remote), agent CLI install, chown workspace (remote), write marker, exec gosu
    install-agent.sh            # Agent CLI install (Claude binary / Codex npm) with version pinning
    install-runtimes.sh         # Language runtime install (Python, Go, Rust, Ruby, Lua)
    user-entrypoint.sh          # User-phase (PID 1): env forwarding, git config, heartbeat, handle orphaned terminals (remote)
    create-terminal.sh          # Agent-aware terminal creation (branches on WARDEN_AGENT_TYPE)
    create-shell.sh             # Lazy, idempotent bootstrap for the auxiliary bash-shell tmux session (Terminal tab)
    disconnect-terminal.sh      # Disconnect viewer, tmux session continues
    kill-worktree.sh            # Kill the agent + shell tmux sessions and all processes for a worktree
    warden-write-event.sh       # Shared atomic event file write library
    warden-push-event.sh        # Terminal lifecycle event helper
    warden-heartbeat.sh         # Background heartbeat (every 10s)
    warden-network-block-logger.sh # Polls xt_recent for blocked IPs, writes audit events
    setup-network-isolation.sh  # iptables OUTPUT rules for network modes
    install-clipboard-shim.sh   # Install xclip wrapper for web terminal image paste
  claude/
    warden-event-claude.sh      # Claude attention state dispatcher (notification, pre_tool_use, user_prompt)
  codex/
    warden-event-codex.sh       # Placeholder (Codex has no hooks yet)
```

## Install Scripts

The Dockerfile calls each install script as a separate `RUN` instruction for layer caching. The devcontainer feature path calls `install-tools.sh` which orchestrates all sub-scripts in order.

| Script                   | Layer | Changes when...                           |
| ------------------------ | ----- | ----------------------------------------- |
| `install-system-deps.sh` | 1     | New system packages added                 |
| `install-user.sh`        | 2     | User setup or env forwarding changes      |
| `install-warden.sh`      | 3     | Any Warden script changes (most frequent) |

Agent CLIs are installed at container startup (not at build time) by `install-agent.sh`. Language runtimes are installed by `install-runtimes.sh`. Both use the `warden-cache` volume for caching.

## Terminal Lifecycle

### entrypoint.sh

Root-phase entrypoint. For local projects: sets up UID remapping and drops privileges via gosu. For remote projects: clones the repository into the workspace volume first, then chowns the workspace to match the warden user. Both paths: installs agent CLI and language runtimes, writes `/tmp/warden-installs-done` marker to signal the Go server that network isolation can be applied (only after downloads are complete), then `exec gosu warden` to permanently drop to the warden user for the user-phase entrypoint.

### create-terminal.sh

Accepts `<worktree-id> [--skip-permissions]`. Branches on `WARDEN_AGENT_TYPE` env var:

- **Auto-resume detection**: before building the agent command, checks if `exit_code` exists (from a prior session) and JSONL session files are present. If so, appends `--continue` (claude-code) or uses `codex resume --last` (codex) to resume the previous conversation. Also builds a fresh command (without resume flag) for fallback.
- **claude-code** (default): launches `claude --worktree <id>` in `.claude/worktrees/<id>/` (Claude manages worktrees natively). Adds `--dangerously-skip-permissions` if requested.
- **codex**: creates the git worktree manually (`git worktree add`) in `.warden/worktrees/<id>/` if it doesn't exist, with fallback to use existing branch if worktree creation fails, then launches `codex --no-alt-screen` in the worktree directory (the `--no-alt-screen` flag disables alternate screen mode so terminal scrollback is preserved). Adds `--dangerously-bypass-approvals-and-sandbox` if skip-permissions is requested.

### user-entrypoint.sh

User-phase (PID 1) entrypoint. Forwards environment variables, configures git, and starts the heartbeat background process. For remote projects: scans for orphaned terminal directories (from containers that were killed without cleanup) and writes exit_code markers so they can be auto-resumed on reconnect. For non-full network modes: starts `warden-network-block-logger.sh` to poll for blocked IPs and write audit events. Note: socat socket bridges for SSH/GPG agent forwarding are now managed by the Go server via `docker exec` (see `engine/bridge_exec.go` and `service/socket_bridge.go`) rather than being started from env vars in this script.

### create-terminal.sh

Unsets `TMUX` env var in the inner script so agents don't detect they're inside tmux. If the agent exits with a non-zero code and auto-resume was attempted, falls back to a fresh session (prevents being dumped into bare bash when there's no conversation to resume). When the agent exits: records exit code, pushes `session_exit` event, drops to `exec bash`.

### create-shell.sh

Accepts `<worktree-id>`. Lazily creates `warden-shell-<wid>` — a plain bash tmux session at the worktree's working directory — for the webapp's **Terminal** tab and the TUI's `s` (shell) keybind. Idempotent: exits 0 if the session already exists. Independent from the agent session (no auto-resume, no JSONL, no `exit_code` file, no lifecycle events). Configured with the same tmux options as `create-terminal.sh` so the xterm.js viewer behaves identically.

### disconnect-terminal.sh

Pushes `terminal_disconnected` event. Tmux session continues running.

### kill-worktree.sh

Writes `exit_code=137` if not already present (for auto-resume on reconnect), then kills both the agent tmux session (`warden-<wid>`) and the auxiliary shell tmux session (`warden-shell-<wid>`). Pushes `process_killed` event. Cleans up stale tracking files but preserves exit_code.

## Event Scripts

### warden-write-event.sh

Shared library sourced by all event-producing scripts. Provides:

- `warden_extract_field "$json" "field"` — bash regex extraction for simple string values (no jq fork)
- `warden_build_event_json "$type" "$data"` — constructs event envelope JSON (requires `CONTAINER_NAME`, `PROJECT_ID`, `WORKTREE_ID`)
- `warden_write_event "$json"` — atomic write to event directory (`.tmp` → `.json` rename). Extracts worktree ID from `.claude/worktrees/` and `.warden/worktrees/` paths.

### warden-heartbeat.sh

Background process (started by entrypoint.sh). Writes heartbeat event every 10s for backend liveness detection.

### warden-network-block-logger.sh

Background process (started by user-entrypoint.sh for non-full network modes). Polls `/proc/net/xt_recent/warden_blocked` every 30s for destination IPs blocked by iptables. Resolves IPs to hostnames via reverse DNS and writes `network_blocked` events to the audit log. Deduplicates by IP — each blocked destination is reported once per container lifetime. Exits silently if `xt_recent` is not available.

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

## Clipboard Integration

### install-clipboard-shim.sh

Installs an xclip wrapper shim at `~/.local/bin/xclip` that intercepts clipboard operations for web terminal image paste support. Images can be pasted via Ctrl+V (native paste event) or dragged and dropped onto the terminal. Non-PNG images are auto-converted to PNG before staging (both Claude Code and Codex expect PNG).

When the web frontend uploads an image:

1. Browser uploads image via `POST /api/v1/projects/{projectId}/{agentType}/clipboard`
2. Service stages the file in `/tmp/warden-clipboard/`
3. Agent-specific paste behavior triggers (see below)
4. Shim cleans up stale files older than 5 minutes

**Agent-specific behavior:**

- **Claude Code**: Uses the xclip shim. User sends Ctrl+V to the terminal, Claude calls `xclip` to read the clipboard, and the shim returns the staged image (via TARGETS or image read calls).
- **Codex**: Receives the staged file path as text input (since Codex uses arboard for clipboard access, which requires X11 — not available in containers). The file path is sent directly to the terminal's stdin.

The xclip shim falls back to the real xclip binary for all non-image clipboard operations.

## Attention Tracking

Claude Code's hooks push attention state to the event bus via `claude/warden-event-claude.sh`. Attention types from the Notification hook:

- `permission_prompt` — Claude needs tool approval
- `idle_prompt` — Claude is done, waiting for the next prompt
- `elicitation_dialog` — Claude is asking the user a question

`UserPromptSubmit` and `PreToolUse` hooks push attention-clear events when the user responds. `PreToolUse` with `tool_name == "AskUserQuestion"` pushes `needs_answer` instead.

Codex attention tracking is a known upstream gap — no hooks available yet.

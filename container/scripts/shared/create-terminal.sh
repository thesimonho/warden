#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Create a terminal for a worktree inside the container.
#
# Usage: create-terminal.sh <worktree-id> [--skip-permissions]
#
# Starts a tmux session running the configured agent (Claude Code
# or Codex) for the given worktree. When the agent exits, the bash
# shell stays alive so the user can run follow-up commands.
#
# The worktree-id is either a git worktree name (for git repos) or
# "main" for non-git repos (the workspace root).
#
# The Go backend connects to this tmux session via docker exec with
# TTY mode, proxied over WebSocket to the browser's xterm.js.
#
# Outputs JSON to stdout: {"worktreeId":"..."}
# -------------------------------------------------------------------

WORKTREE_ID="${1:?Usage: create-terminal.sh <worktree-id> [--skip-permissions]}"
SKIP_PERMISSIONS="${2:-}"

# -------------------------------------------------------------------
# Validate worktree ID (alphanumeric, hyphens, underscores, dots)
# -------------------------------------------------------------------
if [[ ! "$WORKTREE_ID" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
  echo '{"error":"invalid worktree ID"}' >&2
  exit 1
fi

WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
TERMINAL_DIR="${WORKSPACE_DIR}/.warden/terminals/${WORKTREE_ID}"
mkdir -p "$TERMINAL_DIR"

# -------------------------------------------------------------------
# Determine working directory and agent type
# -------------------------------------------------------------------
IS_GIT_REPO=false
WORK_DIR="$WORKSPACE_DIR"
AGENT_TYPE="${WARDEN_AGENT_TYPE:-claude-code}"

if git -C "$WORKSPACE_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  IS_GIT_REPO=true
fi

# -------------------------------------------------------------------
# Auto-resume: detect previous session via exit_code + JSONL files.
# Only resume if the agent exited normally (exit_code file exists
# from a prior run). Do NOT resume if the worktree was explicitly
# killed (no exit_code, terminal dir is fresh).
# -------------------------------------------------------------------
RESUME_FLAG=""
if [ -f "${TERMINAL_DIR}/exit_code" ]; then
  case "$AGENT_TYPE" in
    claude-code)
      if find ~/.claude/projects -maxdepth 2 -name '*.jsonl' -print -quit 2>/dev/null | grep -q .; then
        RESUME_FLAG="--continue"
      fi
      ;;
    codex)
      if find ~/.codex/sessions -maxdepth 4 -name '*.jsonl' -print -quit 2>/dev/null | grep -q .; then
        RESUME_FLAG="resume --last"
      fi
      ;;
  esac
fi

# Clear stale state from any previous run
rm -f "${TERMINAL_DIR}/exit_code"

# -------------------------------------------------------------------
# Build the agent command based on agent type.
#
# Claude Code: uses --worktree flag to manage worktrees natively.
# Codex: has no --worktree flag. Warden creates the git worktree and
#   sets the working directory before launching Codex.
# -------------------------------------------------------------------
case "$AGENT_TYPE" in
  codex)
    # Codex has no --worktree flag — create the worktree manually and
    # set the working directory to the worktree path.
    if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
      WORKTREE_PATH="${WORKSPACE_DIR}/.warden/worktrees/${WORKTREE_ID}"
      if [ ! -d "$WORKTREE_PATH" ]; then
        git -C "$WORKSPACE_DIR" worktree add "$WORKTREE_PATH" -b "$WORKTREE_ID" >/dev/null 2>&1 \
          || git -C "$WORKSPACE_DIR" worktree add "$WORKTREE_PATH" "$WORKTREE_ID" >/dev/null 2>&1 \
          || true
      fi
      if [ -d "$WORKTREE_PATH" ]; then
        WORK_DIR="$WORKTREE_PATH"
      else
        echo '{"error":"failed to create git worktree"}' >&2
        exit 1
      fi
    fi

    # Build the fresh command (no resume flag).
    AGENT_CMD_FRESH="codex --no-alt-screen"
    if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
      AGENT_CMD_FRESH="${AGENT_CMD_FRESH} --dangerously-bypass-approvals-and-sandbox"
    fi

    # resume is a subcommand — must come before flags
    if [ -n "$RESUME_FLAG" ]; then
      AGENT_CMD="codex ${RESUME_FLAG} --no-alt-screen"
      if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
        AGENT_CMD="${AGENT_CMD} --dangerously-bypass-approvals-and-sandbox"
      fi
    else
      AGENT_CMD="$AGENT_CMD_FRESH"
      AGENT_CMD_FRESH=""
    fi
    ;;

  *) # claude-code (default)
    # Build the fresh command (no resume flag).
    AGENT_CMD_FRESH="claude"
    if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
      AGENT_CMD_FRESH="${AGENT_CMD_FRESH} --worktree '${WORKTREE_ID}'"
    fi
    if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
      AGENT_CMD_FRESH="${AGENT_CMD_FRESH} --dangerously-skip-permissions"
    fi

    if [ -n "$RESUME_FLAG" ]; then
      AGENT_CMD="${AGENT_CMD_FRESH} ${RESUME_FLAG}"
    else
      AGENT_CMD="$AGENT_CMD_FRESH"
      AGENT_CMD_FRESH=""
    fi
    ;;
esac

# -------------------------------------------------------------------
# Build the inner script to run inside tmux.
#
# Written to a file instead of a bash -c string to avoid nested
# quoting issues (single quotes in the command break when placed
# inside bash -c '...'). The script launches the agent interactively.
# When the user exits, we capture the exit code, push a session_exit
# event, and drop to a bash shell so the tmux session stays alive
# for follow-up work (inspect output, run commands, etc).
# -------------------------------------------------------------------
INNER_SCRIPT="${TERMINAL_DIR}/inner-cmd.sh"
cat > "$INNER_SCRIPT" << EOF
#!/usr/bin/env bash
# Unset TMUX so agents don't detect they're inside tmux. This prevents
# Claude Code from showing tmux-specific hints and ensures agents
# behave as if they're not inside tmux (no TMUX env var).
unset TMUX
cd '${WORK_DIR}' && ${AGENT_CMD}
EXIT_CODE=\$?
# If auto-resume failed (e.g. no conversation to continue), fall back
# to a fresh session so the user doesn't get dumped into bare bash.
if [ \$EXIT_CODE -ne 0 ] && [ -n '${AGENT_CMD_FRESH}' ]; then
  ${AGENT_CMD_FRESH}
  EXIT_CODE=\$?
fi
echo \$EXIT_CODE > '${TERMINAL_DIR}/exit_code'
/usr/local/bin/warden-push-event.sh session_exit '${WORKTREE_ID}' "{\"exitCode\":\$EXIT_CODE}"
exec bash
EOF
chmod +x "$INNER_SCRIPT"

# -------------------------------------------------------------------
# Start tmux as a detached session.
#
# tmux new-session -d is inherently detached — no nohup or
# backgrounding needed. The Go backend attaches to this session via
# docker exec with TTY mode using "tmux attach-session -t".
#
# Per-session options:
#   status off      — hide the status bar (terminal is full-screen)
#   mouse off       — let xterm.js handle mouse events
#   history-limit   — 50000 lines of scrollback for replay on reconnect
#
# Uses bash -l so the login environment (PATH, .docker_env) is loaded.
# -------------------------------------------------------------------
SESSION_NAME="warden-${WORKTREE_ID}"

bash -lc "tmux -u new-session -d -s '${SESSION_NAME}' -x 200 -y 50 bash '${INNER_SCRIPT}'"

# Configure session for embedded use.
# window-size=latest: resize to the most recently attached client's
# dimensions so the terminal matches the browser viewport on connect.
tmux set-option -t "${SESSION_NAME}" window-size latest 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" status off 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" mouse off 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" history-limit 50000 2>/dev/null || true

# Push terminal_connected event to the event bus
/usr/local/bin/warden-push-event.sh terminal_connected "$WORKTREE_ID" &

# -------------------------------------------------------------------
# Output result for the Go backend to parse
# -------------------------------------------------------------------
echo "{\"worktreeId\":\"${WORKTREE_ID}\"}"

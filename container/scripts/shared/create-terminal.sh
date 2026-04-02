#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Create a terminal for a worktree inside the container.
#
# Usage: create-terminal.sh <worktree-id> [--skip-permissions]
#
# Starts an abduco session running the configured agent (Claude Code
# or Codex) for the given worktree. When the agent exits, the bash
# shell stays alive so the user can run follow-up commands.
#
# The worktree-id is either a git worktree name (for git repos) or
# "main" for non-git repos (the workspace root).
#
# The Go backend connects to this abduco session via docker exec with
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

    AGENT_CMD="codex --no-alt-screen"
    if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
      AGENT_CMD="codex --no-alt-screen --dangerously-bypass-approvals-and-sandbox"
    fi
    ;;

  *) # claude-code (default)
    AGENT_CMD="claude"
    if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
      AGENT_CMD="${AGENT_CMD} --worktree '${WORKTREE_ID}'"
    fi
    if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
      AGENT_CMD="${AGENT_CMD} --dangerously-skip-permissions"
    fi
    ;;
esac

# -------------------------------------------------------------------
# Build the command to run inside abduco.
#
# Launches the agent interactively. When the user exits, we write the
# exit status and drop to a bash shell so the abduco session stays
# alive for follow-up work (inspect output, run commands, etc).
# -------------------------------------------------------------------
INNER_CMD="cd '${WORK_DIR}' && ${AGENT_CMD}; \
  EXIT_CODE=\$?; \
  echo \$EXIT_CODE > '${TERMINAL_DIR}/exit_code'; \
  /usr/local/bin/warden-push-event.sh session_exit '${WORKTREE_ID}' '{\"exitCode\":'\$EXIT_CODE'}'; \
  exec bash"

# -------------------------------------------------------------------
# Start abduco as a detached daemon (-n: create session, don't attach).
#
# abduco holds the PTY alive across viewer disconnections. The Go
# backend attaches to this session via docker exec with TTY mode
# using "abduco -a" (separate attach command with a real TTY).
#
# IMPORTANT: Do NOT use -A (create+attach). With -A, abduco enters
# client_mainloop reading from stdin. Since nohup redirects stdin to
# /dev/null, pselect() returns immediately every iteration (dev/null
# is always readable, read returns 0), causing a 100% CPU busy-wait.
# Uses bash -l so the login environment (PATH, .docker_env) is loaded.
# -------------------------------------------------------------------
nohup bash -lc "exec abduco -n 'warden-${WORKTREE_ID}' bash -c '${INNER_CMD}'" \
  > /dev/null 2>&1 &

# Give abduco a moment to start the session
sleep 0.3

# Push terminal_connected event to the event bus
/usr/local/bin/warden-push-event.sh terminal_connected "$WORKTREE_ID" &

# -------------------------------------------------------------------
# Output result for the Go backend to parse
# -------------------------------------------------------------------
echo "{\"worktreeId\":\"${WORKTREE_ID}\"}"

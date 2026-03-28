#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Create a terminal for a worktree inside the container.
#
# Usage: create-terminal.sh <worktree-id> [--skip-permissions]
#
# Starts an abduco session running Claude Code with --worktree for git
# repos. When Claude exits, the bash shell stays alive so the user can
# run follow-up commands.
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
TERMINAL_DIR="${WORKSPACE_DIR}/.warden-terminals/${WORKTREE_ID}"
mkdir -p "$TERMINAL_DIR"

# -------------------------------------------------------------------
# Determine working directory and Claude flags
# -------------------------------------------------------------------
IS_GIT_REPO=false
WORK_DIR="$WORKSPACE_DIR"

if git -C "$WORKSPACE_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  IS_GIT_REPO=true
fi

# Clear stale state from any previous run
rm -f "${TERMINAL_DIR}/exit_code"

# -------------------------------------------------------------------
# Build the command to run inside abduco
#
# Launches interactive Claude Code. When the user exits Claude, we
# write the exit status and drop to a bash shell so the abduco session
# stays alive for follow-up work (inspect output, run commands, etc).
# -------------------------------------------------------------------
CLAUDE_FLAGS=""
if [ "$IS_GIT_REPO" = true ] && [ "$WORKTREE_ID" != "main" ]; then
  CLAUDE_FLAGS="--worktree '${WORKTREE_ID}'"
fi
if [ "$SKIP_PERMISSIONS" = "--skip-permissions" ]; then
  CLAUDE_FLAGS="--dangerously-skip-permissions ${CLAUDE_FLAGS}"
fi

INNER_CMD="cd '${WORK_DIR}' && claude ${CLAUDE_FLAGS}; \
  EXIT_CODE=\$?; \
  echo \$EXIT_CODE > '${TERMINAL_DIR}/exit_code'; \
  /usr/local/bin/warden-capture-cost.sh '${WORKTREE_ID}'; \
  /usr/local/bin/warden-push-event.sh session_exit '${WORKTREE_ID}' '{\"exitCode\":'\$EXIT_CODE'}'; \
  exec bash"

# -------------------------------------------------------------------
# Start abduco detached via nohup.
#
# abduco holds the PTY alive across viewer disconnections. The Go
# backend attaches to this session via docker exec with TTY mode.
# Uses bash -l so the login environment (PATH, .docker_env) is loaded.
# -------------------------------------------------------------------
nohup bash -lc "exec abduco -A 'warden-${WORKTREE_ID}' bash -c '${INNER_CMD}'" \
  > /dev/null 2>&1 &

# Give abduco a moment to start the session
sleep 0.3

# Push terminal_connected event to the event bus
/usr/local/bin/warden-push-event.sh terminal_connected "$WORKTREE_ID" &

# -------------------------------------------------------------------
# Output result for the Go backend to parse
# -------------------------------------------------------------------
echo "{\"worktreeId\":\"${WORKTREE_ID}\"}"

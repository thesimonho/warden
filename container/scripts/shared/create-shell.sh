#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Create (or reuse) the auxiliary bash-shell tmux session for a worktree.
#
# Usage: create-shell.sh <worktree-id>
#
# Backs the Terminal tab in the webapp and the shell attach action in
# the TUI. Idempotent — if the shell session already exists, this
# script is a no-op. The first attach creates it lazily, and subsequent
# attaches reconnect to the same live session.
#
# This session is independent from the agent session created by
# create-terminal.sh:
#   - Name prefix: `warden-shell-` (vs `warden-` for the agent).
#   - No auto-resume, no exit_code file, no session_exit events.
#   - Not tracked as an agent connection (no terminal_connected event).
#   - Killed only when the worktree is reset or deleted (via
#     kill-worktree.sh killing both the agent and shell sessions).
#
# The worktree-id is either a git worktree name (for git repos) or
# "main" for non-git repos (the workspace root).
# -------------------------------------------------------------------

WORKTREE_ID="${1:?Usage: create-shell.sh <worktree-id>}"

# -------------------------------------------------------------------
# Validate worktree ID (same rules as create-terminal.sh)
# -------------------------------------------------------------------
if [[ ! "$WORKTREE_ID" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
  echo '{"error":"invalid worktree ID"}' >&2
  exit 1
fi

SESSION_NAME="warden-shell-${WORKTREE_ID}"

# -------------------------------------------------------------------
# Idempotent: if the session already exists, nothing to do.
# -------------------------------------------------------------------
if tmux -u has-session -t "$SESSION_NAME" 2>/dev/null; then
  echo "{\"worktreeId\":\"${WORKTREE_ID}\",\"status\":\"exists\"}"
  exit 0
fi

# -------------------------------------------------------------------
# Determine working directory.
#
# For git repos with a non-main worktree, reuse the same worktree path
# that create-terminal.sh uses (for Codex) or that Claude Code would
# create via --worktree. If the worktree directory already exists (from
# a previous agent launch), we attach to it directly. If it does not
# exist yet, we fall back to the workspace root — the agent will
# materialise the worktree on its first launch, and subsequent shell
# attaches will land in the real worktree directory.
# -------------------------------------------------------------------
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
WORK_DIR="$WORKSPACE_DIR"

if [ "$WORKTREE_ID" != "main" ] \
  && git -C "$WORKSPACE_DIR" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  WORKTREE_PATH="${WORKSPACE_DIR}/.warden/worktrees/${WORKTREE_ID}"
  if [ -d "$WORKTREE_PATH" ]; then
    WORK_DIR="$WORKTREE_PATH"
  fi
fi

# -------------------------------------------------------------------
# Start the tmux session detached, running a login bash at WORK_DIR.
# Matches create-terminal.sh's tmux configuration so the xterm.js
# viewer behaves identically (scrollback, clipboard, resize).
# -------------------------------------------------------------------
bash -lc "tmux -u new-session -d -s '${SESSION_NAME}' -c '${WORK_DIR}' -x 200 -y 50 bash -l"

tmux set-option -t "${SESSION_NAME}" window-size latest 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" status off 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" mouse off 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" history-limit 50000 2>/dev/null || true
tmux set-option -t "${SESSION_NAME}" set-clipboard on 2>/dev/null || true

echo "{\"worktreeId\":\"${WORKTREE_ID}\",\"status\":\"created\"}"

#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Kill a worktree's processes inside the container.
#
# Usage: kill-worktree.sh <worktree-id>
#
# Kills the tmux session and everything running inside it (Claude
# Code, bash shell). Removes the terminal tracking directory entry.
#
# This does NOT remove the git worktree directory — worktrees persist
# on disk independently. Cleanup is a git operation.
# -------------------------------------------------------------------

WORKTREE_ID="${1:?Usage: kill-worktree.sh <worktree-id>}"

# Validate worktree ID
if [[ ! "$WORKTREE_ID" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
  echo '{"error":"invalid worktree ID"}' >&2
  exit 1
fi

WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
TERMINAL_DIR="${WORKSPACE_DIR}/.warden/terminals/${WORKTREE_ID}"

# -------------------------------------------------------------------
# Write exit_code if not already present so auto-resume can recover
# the session on the next connect. The agent's inner script writes
# exit_code on normal exit; this covers the kill-while-running case.
# -------------------------------------------------------------------
mkdir -p "$TERMINAL_DIR"
if [ ! -f "${TERMINAL_DIR}/exit_code" ]; then
  echo "137" > "${TERMINAL_DIR}/exit_code"
fi

# -------------------------------------------------------------------
# Kill tmux sessions: the agent session AND the auxiliary bash-shell
# session (created lazily by create-shell.sh for the Terminal tab).
# Both are tied to the worktree lifetime — Reset / Delete tears both
# down so no stray processes remain.
# -------------------------------------------------------------------
AGENT_SESSION="warden-${WORKTREE_ID}"
SHELL_SESSION="warden-shell-${WORKTREE_ID}"
tmux -u kill-session -t "$AGENT_SESSION" 2>/dev/null || true
tmux -u kill-session -t "$SHELL_SESSION" 2>/dev/null || true

# -------------------------------------------------------------------
# Clean up tracking state but preserve exit_code for auto-resume.
# The exit_code file signals that a previous session can be resumed
# via --continue / resume --last on the next connect. The terminal
# dir itself is only fully removed on worktree deletion (handled by
# the Go engine's RemoveWorktree).
# -------------------------------------------------------------------
rm -f "${TERMINAL_DIR}/port" "${TERMINAL_DIR}/inner-cmd.sh"

# Push process_killed event to the event bus
/usr/local/bin/warden-push-event.sh process_killed "$WORKTREE_ID" &

echo '{"status":"killed"}'

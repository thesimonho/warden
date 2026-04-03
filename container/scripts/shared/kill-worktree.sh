#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Kill a worktree's processes inside the container.
#
# Usage: kill-worktree.sh <worktree-id>
#
# Kills the abduco session and everything running inside it (Claude
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
# Kill abduco session
# -------------------------------------------------------------------
# Match both -n (new) and -A (legacy) flags
pkill -f "abduco -[nA] .*warden-${WORKTREE_ID}" 2>/dev/null || true

# -------------------------------------------------------------------
# Remove terminal tracking directory entry
# -------------------------------------------------------------------
rm -rf "$TERMINAL_DIR"

# Push process_killed event to the event bus
/usr/local/bin/warden-push-event.sh process_killed "$WORKTREE_ID" &

echo '{"status":"killed"}'

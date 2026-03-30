#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Disconnect a terminal viewer from a worktree inside the container.
#
# Usage: disconnect-terminal.sh <worktree-id>
#
# With the WebSocket proxy architecture, the Go backend manages viewer
# connections via docker exec. This script just pushes the disconnect
# event and cleans up tracking state. The abduco session and everything
# running inside it (Claude, bash) continue running in the background.
#
# Use kill-worktree.sh to kill abduco and destroy the process entirely.
# -------------------------------------------------------------------

WORKTREE_ID="${1:?Usage: disconnect-terminal.sh <worktree-id>}"

# Validate worktree ID
if [[ ! "$WORKTREE_ID" =~ ^[a-zA-Z0-9][a-zA-Z0-9._-]*$ ]]; then
  echo '{"error":"invalid worktree ID"}' >&2
  exit 1
fi

WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
TERMINAL_DIR="${WORKSPACE_DIR}/.warden-terminals/${WORKTREE_ID}"

# Clean up any stale port file from the old ttyd architecture
rm -f "${TERMINAL_DIR}/port"

# Push terminal_disconnected event to the event bus
/usr/local/bin/warden-push-event.sh terminal_disconnected "$WORKTREE_ID" &

echo '{"status":"disconnected"}'

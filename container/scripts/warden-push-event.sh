#!/usr/bin/env bash
# -------------------------------------------------------------------
# Shared helper for writing terminal lifecycle events to the Warden
# event directory.
#
# Usage: warden-push-event.sh <type> <worktree-id> [json-data]
#
# Environment:
#   WARDEN_CONTAINER_NAME — container name (set by Warden at creation)
#   WARDEN_PROJECT_ID     — deterministic project identifier (set by Warden at creation)
#   WARDEN_EVENT_DIR      — event directory (set by Warden at creation)
#
# Exits silently if the event directory is missing (fire-and-forget).
# -------------------------------------------------------------------
set -euo pipefail

EVENT_TYPE="${1:-}"
WORKTREE_ID="${2:-}"
DATA="${3:-{}}"

if [ -z "$EVENT_TYPE" ] || [ -z "$WORKTREE_ID" ]; then
  exit 0
fi

CONTAINER_NAME="${WARDEN_CONTAINER_NAME:-}"
if [ -z "$CONTAINER_NAME" ]; then
  exit 0
fi

PROJECT_ID="${WARDEN_PROJECT_ID:-}"

if [ -z "${WARDEN_EVENT_DIR:-}" ]; then
  exit 0
fi

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

JSON=$(jq -cn \
  --arg type "$EVENT_TYPE" \
  --arg cn "$CONTAINER_NAME" \
  --arg pid "$PROJECT_ID" \
  --arg wt "$WORKTREE_ID" \
  --argjson data "$DATA" \
  '{"type": $type, "containerName": $cn, "projectId": $pid, "worktreeId": $wt, "data": $data}')

warden_write_event "$JSON"

exit 0

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
DEFAULT_DATA='{}'
DATA="${3:-$DEFAULT_DATA}"

if [ -z "$EVENT_TYPE" ] || [ -z "$WORKTREE_ID" ]; then
  exit 0
fi

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

warden_check_event_env || exit 0

warden_write_event "$(warden_build_event_json "$EVENT_TYPE" "$DATA")"

exit 0

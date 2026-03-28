#!/usr/bin/env bash
# -------------------------------------------------------------------
# Capture cost from Claude Code's .claude.json and fire a stop event.
#
# Called after Claude exits (including Ctrl-C) to ensure cost data is
# persisted even when Claude Code's hooks are cancelled. By the time
# this runs, Claude has already written .claude.json.
#
# Usage: warden-capture-cost.sh <worktree-id>
#
# Environment:
#   WARDEN_CONTAINER_NAME — container name (set by Warden at creation)
#   WARDEN_PROJECT_ID     — deterministic project identifier (set by Warden at creation)
#   WARDEN_EVENT_DIR      — event directory (set by Warden at creation)
#   WARDEN_WORKSPACE_DIR  — workspace directory inside the container
# -------------------------------------------------------------------
set -euo pipefail

WORKTREE_ID="${1:-}"
CONTAINER_NAME="${WARDEN_CONTAINER_NAME:-}"
PROJECT_ID="${WARDEN_PROJECT_ID:-}"
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"

if [ -z "$WORKTREE_ID" ] || [ -z "${WARDEN_EVENT_DIR:-}" ] || [ -z "$CONTAINER_NAME" ]; then
  exit 0
fi

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh
# shellcheck source=warden-cost-lib.sh
source /usr/local/bin/warden-cost-lib.sh

send_cost_event

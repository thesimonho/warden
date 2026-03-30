#!/usr/bin/env bash
# -------------------------------------------------------------------
# Warden heartbeat — periodically writes a heartbeat event so the
# backend can detect container/process crashes when no hook fires.
#
# Started as a background process from entrypoint.sh. Writes a minimal
# heartbeat event every 10 seconds. Exits silently if the event
# directory is not configured.
#
# Environment:
#   WARDEN_CONTAINER_NAME — container name (set by Warden at creation)
#   WARDEN_PROJECT_ID     — deterministic project identifier (set by Warden at creation)
#   WARDEN_EVENT_DIR      — event directory (set by Warden at creation)
# -------------------------------------------------------------------
set -euo pipefail

INTERVAL=10

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

while true; do
  sleep "$INTERVAL"

  WORKTREE_ID=""
  warden_write_event "$(warden_build_event_json "heartbeat" "{}")"
done

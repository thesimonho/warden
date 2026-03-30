#!/usr/bin/env bash
# -------------------------------------------------------------------
# Claude Code attention state dispatcher — writes real-time attention
# events to the bind-mounted event directory for the host-side watcher.
#
# With the JSONL session parser as the primary data source, this script
# only handles events that are NOT available in the JSONL session file:
#   - notification → attention state (permission prompts, idle, etc.)
#   - pre_tool_use → AskUserQuestion detection (needs_answer state)
#   - user_prompt_submit → attention_clear + user_prompt audit event
#
# All other events (session lifecycle, tool use, cost, etc.) are parsed
# from the JSONL session file by the Go backend.
#
# Called by Claude Code hooks via managed settings at
# /etc/claude-code/managed-settings.json. Reads hook JSON from stdin.
#
# Environment:
#   WARDEN_CONTAINER_NAME — container name (set by Warden at creation)
#   WARDEN_PROJECT_ID     — deterministic project identifier
#   WARDEN_EVENT_DIR      — event directory (set by Warden at creation)
# -------------------------------------------------------------------
set -euo pipefail

EVENT_TYPE="${1:-}"

if [ -z "$EVENT_TYPE" ]; then
  exit 0
fi

# shellcheck source=../shared/warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

warden_check_event_env || exit 0

INPUT=$(cat)

CWD=$(warden_extract_field "$INPUT" "cwd")
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
WORKTREE_ID=$(warden_extract_worktree_id "$CWD" "$WORKSPACE_DIR")

# -------------------------------------------------------------------
# Handle attention state events.
# -------------------------------------------------------------------
DATA="{}"

case "$EVENT_TYPE" in
  notification)
    NOTIFICATION_TYPE=$(warden_extract_field "$INPUT" "notification_type")
    if [ -n "$NOTIFICATION_TYPE" ]; then
      DATA="{\"notificationType\":\"${NOTIFICATION_TYPE}\"}"
    fi
    EVENT_TYPE="attention"
    ;;

  pre_tool_use)
    TOOL_NAME=$(warden_extract_field "$INPUT" "tool_name")
    if [ "$TOOL_NAME" = "AskUserQuestion" ]; then
      EVENT_TYPE="needs_answer"
    else
      EVENT_TYPE="attention_clear"
    fi
    DATA="{\"toolName\":\"${TOOL_NAME}\"}"
    ;;

  user_prompt_submit)
    warden_write_event "$(warden_build_event_json "attention_clear" "{}")"

    # Filter out system-injected messages to avoid polluting the audit log.
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty' 2>/dev/null)
    TRIMMED=$(echo "$PROMPT" | sed 's/^[[:space:]]*//')
    if echo "$TRIMMED" | grep -qE '^<(task-notification|user-prompt-submit-hook)>'; then
      exit 0
    fi

    EVENT_TYPE="user_prompt"
    DATA=$(jq -cn --arg prompt "$(printf '%.500s' "$PROMPT")" '{"prompt": $prompt}')
    ;;

  *)
    # Unknown or JSONL-handled event type — ignore silently.
    exit 0
    ;;
esac

warden_write_event "$(warden_build_event_json "$EVENT_TYPE" "$DATA")"

exit 0

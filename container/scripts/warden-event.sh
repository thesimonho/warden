#!/usr/bin/env bash
# -------------------------------------------------------------------
# Warden event dispatcher — writes Claude Code hook events to the
# bind-mounted event directory for the host-side watcher.
#
# Called by Claude Code hooks via managed settings at
# /etc/claude-code/managed-settings.json. Reads hook JSON from stdin,
# extracts the worktree ID and event-specific data, and writes a JSON
# event file atomically to WARDEN_EVENT_DIR.
#
# Usage: warden-event.sh <event-type>
#   Reads hook JSON from stdin.
#
# Environment:
#   WARDEN_CONTAINER_NAME — container name (set by Warden at creation)
#   WARDEN_PROJECT_ID     — deterministic project identifier (set by Warden at creation)
#   WARDEN_EVENT_DIR      — event directory (set by Warden at creation)
#
# If the event directory is missing, exits silently — this allows
# containers without event delivery to continue working.
# -------------------------------------------------------------------
set -euo pipefail

EVENT_TYPE="${1:-}"

# Require event type argument.
if [ -z "$EVENT_TYPE" ]; then
  exit 0
fi

# Require container name from environment.
CONTAINER_NAME="${WARDEN_CONTAINER_NAME:-}"
if [ -z "$CONTAINER_NAME" ]; then
  exit 0
fi

# Project ID from environment (deterministic hash of host path).
PROJECT_ID="${WARDEN_PROJECT_ID:-}"

# Exit silently if event directory not configured.
if [ -z "${WARDEN_EVENT_DIR:-}" ]; then
  exit 0
fi

# -------------------------------------------------------------------
# Source shared functions.
# -------------------------------------------------------------------
# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh
# shellcheck source=warden-cost-lib.sh
source /usr/local/bin/warden-cost-lib.sh

# Read hook JSON from stdin.
INPUT=$(cat)

# -------------------------------------------------------------------
# Extract worktree ID from cwd.
# Claude Code stores worktrees at .claude/worktrees/<id>;
# .worktrees/<id> is the legacy path.
# -------------------------------------------------------------------
CWD=$(warden_extract_field "$INPUT" "cwd")
WORKTREE_ID=""

WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"

if [ -n "$CWD" ]; then
  if [[ "$CWD" =~ /\.claude/worktrees/([^/]+) ]]; then
    WORKTREE_ID="${BASH_REMATCH[1]}"
  elif [[ "$CWD" =~ /\.worktrees/([^/]+) ]]; then
    WORKTREE_ID="${BASH_REMATCH[1]}"
  elif [ "$CWD" = "$WORKSPACE_DIR" ] || [[ "$CWD" =~ ^${WORKSPACE_DIR}/ ]]; then
    WORKTREE_ID="main"
  fi
fi

# For session_end, the worktree directory may already be removed.
# Extract worktree ID from transcript_path instead.
if [ "$EVENT_TYPE" = "session_end" ] && [ "$WORKTREE_ID" = "main" ]; then
  TRANSCRIPT_PATH=$(warden_extract_field "$INPUT" "transcript_path")
  if [[ "$TRANSCRIPT_PATH" =~ -project--claude-worktrees-([^/]+)/ ]]; then
    WORKTREE_ID="${BASH_REMATCH[1]}"
  fi
fi

# -------------------------------------------------------------------
# Build event-specific data payload.
# Simple string fields use warden_extract_field (no jq fork). Fields
# with arbitrary content or multiple fields use a single jq call.
# -------------------------------------------------------------------
DATA="{}"

case "$EVENT_TYPE" in
  session_start)
    # Capture cost from the previous session — Claude Code may have
    # written .claude.json before firing this hook (e.g. on resume/clear),
    # even if SessionEnd was cancelled by Ctrl-C.
    send_cost_event

    DATA=$(printf '%s' "$INPUT" | jq -c '{
      sessionId: (.session_id // ""),
      model: (.model // ""),
      source: (.source // "")
    }')
    ;;

  session_end)
    # Fire a cost event before the session_end event — last chance to
    # capture cost data before the .claude.json entry may be cleaned up.
    send_cost_event

    REASON=$(warden_extract_field "$INPUT" "reason")
    if [ -n "$REASON" ]; then
      DATA="{\"reason\":\"${REASON}\"}"
    fi
    ;;

  notification)
    NOTIFICATION_TYPE=$(warden_extract_field "$INPUT" "notification_type")
    if [ -n "$NOTIFICATION_TYPE" ]; then
      DATA="{\"notificationType\":\"${NOTIFICATION_TYPE}\"}"
    fi
    # Map Claude Code's "notification" hook to eventbus.EventAttention.
    EVENT_TYPE="attention"
    ;;

  pre_tool_use)
    TOOL_NAME=$(warden_extract_field "$INPUT" "tool_name")

    # Write a tool_use audit event for every tool invocation (backgrounded
    # to avoid adding latency — the attention event below is synchronous).
    # Single jq call extracts tool_input and builds the data object.
    TOOL_USE_DATA=$(printf '%s' "$INPUT" | jq -c \
      --arg tool "$TOOL_NAME" \
      '{toolName: $tool, toolInput: (.tool_input // "")[:1000]}')
    warden_write_event "$(warden_build_event_json "tool_use" "$TOOL_USE_DATA")" &

    # Also send attention state event (existing behavior).
    if [ "$TOOL_NAME" = "AskUserQuestion" ]; then
      EVENT_TYPE="needs_answer"
    else
      EVENT_TYPE="attention_clear"
    fi
    DATA="{\"toolName\":\"${TOOL_NAME}\"}"
    ;;

  user_prompt_submit)
    # Write attention_clear first (existing real-time state behavior).
    warden_write_event "$(warden_build_event_json "attention_clear" "{}")"

    # Claude Code's UserPromptSubmit hook fires for both real user input and
    # system-injected messages. There's no field to distinguish them — only
    # the prompt text differs. Filter out known system tags to avoid polluting
    # the audit log. If new system tags appear, they'll show up as user_prompt
    # events and can be added here.
    PROMPT=$(echo "$INPUT" | jq -r '.prompt // empty' 2>/dev/null)
    TRIMMED=$(echo "$PROMPT" | sed 's/^[[:space:]]*//')
    if echo "$TRIMMED" | grep -qE '^<(task-notification|user-prompt-submit-hook)>'; then
      exit 0
    fi

    EVENT_TYPE="user_prompt"
    DATA=$(jq -cn --arg prompt "$(printf '%.500s' "$PROMPT")" '{"prompt": $prompt}')
    ;;

  post_tool_use_failure)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      toolName: (.tool_name // ""),
      error: (.error // "")[:500]
    }')
    EVENT_TYPE="tool_use_failure"
    ;;

  stop_failure)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      error: (.error // ""),
      errorDetails: (.error_details // "")
    }')
    ;;

  permission_request)
    TOOL_NAME=$(warden_extract_field "$INPUT" "tool_name")
    if [ -n "$TOOL_NAME" ]; then
      DATA="{\"toolName\":\"${TOOL_NAME}\"}"
    fi
    ;;

  subagent_start|subagent_stop)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      agentId: (.agent_id // ""),
      agentType: (.agent_type // "")
    }')
    ;;

  config_change)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      source: (.source // ""),
      filePath: (.file_path // "")
    }')
    ;;

  instructions_loaded)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      filePath: (.file_path // ""),
      loadReason: (.load_reason // "")
    }')
    ;;

  task_completed)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      taskId: (.task_id // ""),
      taskSubject: (.task_subject // "")
    }')
    ;;

  elicitation)
    MCP_SERVER=$(warden_extract_field "$INPUT" "mcp_server_name")
    if [ -n "$MCP_SERVER" ]; then
      DATA="{\"mcpServerName\":\"${MCP_SERVER}\"}"
    fi
    ;;

  elicitation_result)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      mcpServerName: (.mcp_server_name // ""),
      action: (.action // "")
    }')
    ;;

  stop)
    # Background the entire stop event (cost read + event write) so the
    # script exits immediately and Claude Code can fire the Notification
    # hook without waiting for the slow jq-based cost read from .claude.json.
    # Cost capture has multiple redundant paths, so missing this one on a
    # fast container shutdown is acceptable.
    {
      COST_DATA=$(read_cost_data)
      if [ -n "$COST_DATA" ] && [ "$COST_DATA" != "{}" ]; then
        DATA="$COST_DATA"
      fi
      warden_write_event "$(warden_build_event_json "$EVENT_TYPE" "$DATA")"
    } &
    exit 0
    ;;
esac

# -------------------------------------------------------------------
# Write event to the event directory. Atomic file write with
# nanosecond timestamp filename for rough chronological ordering.
# -------------------------------------------------------------------
warden_write_event "$(warden_build_event_json "$EVENT_TYPE" "$DATA")"

exit 0

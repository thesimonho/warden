#!/usr/bin/env bash
# -------------------------------------------------------------------
# Claude Code attention state dispatcher — writes real-time attention
# events to the bind-mounted event directory for the host-side watcher.
#
# The JSONL session parser handles most audit events (tool use, cost,
# prompts, errors). This script handles two categories:
#
# 1. Real-time attention state (not in JSONL):
#    - notification → attention state (permission prompts, idle, etc.)
#    - pre_tool_use → AskUserQuestion detection (needs_answer state)
#    - user_prompt_submit → attention_clear
#
# 2. Audit events not available in JSONL:
#    - session_end, permission_request, config_change,
#      instructions_loaded, task_completed, elicitation, elicitation_result
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
    # Only emit attention_clear — the user_prompt audit event is parsed
    # from the JSONL session file by the Go backend to avoid duplication.
    EVENT_TYPE="attention_clear"
    ;;

  # -----------------------------------------------------------------
  # Audit-only events (not available in JSONL).
  # Cost capture (send_cost_event) is no longer needed here — cost is
  # parsed from JSONL token_update events by the Go backend.
  # -----------------------------------------------------------------
  session_end)
    REASON=$(warden_extract_field "$INPUT" "reason")
    if [ -n "$REASON" ]; then
      DATA="{\"reason\":\"${REASON}\"}"
    fi
    ;;

  permission_request)
    TOOL_NAME=$(warden_extract_field "$INPUT" "tool_name")
    if [ -n "$TOOL_NAME" ]; then
      DATA="{\"toolName\":\"${TOOL_NAME}\"}"
    fi
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

  subagent_start|subagent_stop)
    DATA=$(printf '%s' "$INPUT" | jq -c '{
      agentId: (.agent_id // ""),
      agentType: (.agent_type // "")
    }')
    ;;

  *)
    # Unknown or JSONL-handled event type — ignore silently.
    exit 0
    ;;
esac

warden_write_event "$(warden_build_event_json "$EVENT_TYPE" "$DATA")"

exit 0

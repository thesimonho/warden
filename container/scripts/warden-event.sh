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
CWD=$(echo "$INPUT" | jq -r '.cwd // empty' 2>/dev/null)
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
  TRANSCRIPT_PATH=$(echo "$INPUT" | jq -r '.transcript_path // empty' 2>/dev/null)
  if [[ "$TRANSCRIPT_PATH" =~ -project--claude-worktrees-([^/]+)/ ]]; then
    WORKTREE_ID="${BASH_REMATCH[1]}"
  fi
fi

# -------------------------------------------------------------------
# Build event-specific data payload.
# -------------------------------------------------------------------
DATA="{}"

case "$EVENT_TYPE" in
  session_start)
    # Capture cost from the previous session — Claude Code may have
    # written .claude.json before firing this hook (e.g. on resume/clear),
    # even if SessionEnd was cancelled by Ctrl-C.
    send_cost_event

    SESSION_ID=$(echo "$INPUT" | jq -r '.session_id // empty' 2>/dev/null)
    MODEL=$(echo "$INPUT" | jq -r '.model // empty' 2>/dev/null)
    SOURCE=$(echo "$INPUT" | jq -r '.source // empty' 2>/dev/null)
    if [ -n "$SESSION_ID" ] || [ -n "$MODEL" ] || [ -n "$SOURCE" ]; then
      DATA=$(jq -cn \
        --arg sid "$SESSION_ID" \
        --arg model "$MODEL" \
        --arg source "$SOURCE" \
        '{"sessionId": $sid, "model": $model, "source": $source}')
    fi
    ;;

  session_end)
    # Fire a cost event before the session_end event — last chance to
    # capture cost data before the .claude.json entry may be cleaned up.
    send_cost_event

    REASON=$(echo "$INPUT" | jq -r '.reason // empty' 2>/dev/null)
    if [ -n "$REASON" ]; then
      DATA=$(jq -cn --arg reason "$REASON" '{"reason": $reason}')
    fi
    ;;

  notification)
    NOTIFICATION_TYPE=$(echo "$INPUT" | jq -r '.notification_type // empty' 2>/dev/null)
    if [ -n "$NOTIFICATION_TYPE" ]; then
      DATA=$(jq -cn --arg nt "$NOTIFICATION_TYPE" '{"notificationType": $nt}')
    fi
    # Map Claude Code's "notification" hook to eventbus.EventAttention.
    EVENT_TYPE="attention"
    ;;

  pre_tool_use)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null)
    TOOL_INPUT=$(echo "$INPUT" | jq -r '.tool_input // empty' 2>/dev/null)
    TOOL_INPUT_TRUNCATED=$(printf '%.1000s' "$TOOL_INPUT")

    # Write a tool_use audit event for every tool invocation (backgrounded
    # to avoid adding latency — the attention event below is synchronous).
    TOOL_USE_DATA=$(jq -cn \
      --arg tool "$TOOL_NAME" \
      --arg input "$TOOL_INPUT_TRUNCATED" \
      '{"toolName": $tool, "toolInput": $input}')
    TOOL_USE_JSON=$(jq -cn \
      --arg type "tool_use" \
      --arg cn "$CONTAINER_NAME" \
      --arg wt "$WORKTREE_ID" \
      --argjson data "$TOOL_USE_DATA" \
      '{"type": $type, "containerName": $cn, "worktreeId": $wt, "data": $data}')
    warden_write_event "$TOOL_USE_JSON" &

    # Also send attention state event (existing behavior).
    if [ "$TOOL_NAME" = "AskUserQuestion" ]; then
      EVENT_TYPE="needs_answer"
    else
      EVENT_TYPE="attention_clear"
    fi
    DATA=$(jq -cn --arg tool "$TOOL_NAME" '{"toolName": $tool}')
    ;;

  user_prompt_submit)
    # Write attention_clear first (existing real-time state behavior).
    CLEAR_JSON=$(jq -cn \
      --arg type "attention_clear" \
      --arg cn "$CONTAINER_NAME" \
      --arg wt "$WORKTREE_ID" \
      --argjson data '{}' \
      '{"type": $type, "containerName": $cn, "worktreeId": $wt, "data": $data}')
    warden_write_event "$CLEAR_JSON"

    # Log the user prompt as a separate event.
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
    TRUNCATED=$(printf '%.500s' "$PROMPT")
    DATA=$(jq -cn --arg prompt "$TRUNCATED" '{"prompt": $prompt}')
    ;;

  post_tool_use_failure)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null)
    ERROR_MSG=$(echo "$INPUT" | jq -r '.error // empty' 2>/dev/null)
    ERROR_TRUNCATED=$(printf '%.500s' "$ERROR_MSG")
    EVENT_TYPE="tool_use_failure"
    DATA=$(jq -cn \
      --arg tool "$TOOL_NAME" \
      --arg error "$ERROR_TRUNCATED" \
      '{"toolName": $tool, "error": $error}')
    ;;

  stop_failure)
    ERROR_MSG=$(echo "$INPUT" | jq -r '.error // empty' 2>/dev/null)
    ERROR_DETAILS=$(echo "$INPUT" | jq -r '.error_details // empty' 2>/dev/null)
    EVENT_TYPE="stop_failure"
    DATA=$(jq -cn \
      --arg error "$ERROR_MSG" \
      --arg details "$ERROR_DETAILS" \
      '{"error": $error, "errorDetails": $details}')
    ;;

  permission_request)
    TOOL_NAME=$(echo "$INPUT" | jq -r '.tool_name // empty' 2>/dev/null)
    EVENT_TYPE="permission_request"
    DATA=$(jq -cn --arg tool "$TOOL_NAME" '{"toolName": $tool}')
    ;;

  subagent_start|subagent_stop)
    AGENT_ID=$(echo "$INPUT" | jq -r '.agent_id // empty' 2>/dev/null)
    AGENT_TYPE=$(echo "$INPUT" | jq -r '.agent_type // empty' 2>/dev/null)
    DATA=$(jq -cn \
      --arg id "$AGENT_ID" \
      --arg type "$AGENT_TYPE" \
      '{"agentId": $id, "agentType": $type}')
    ;;

  config_change)
    CONFIG_SOURCE=$(echo "$INPUT" | jq -r '.source // empty' 2>/dev/null)
    FILE_PATH=$(echo "$INPUT" | jq -r '.file_path // empty' 2>/dev/null)
    EVENT_TYPE="config_change"
    DATA=$(jq -cn \
      --arg source "$CONFIG_SOURCE" \
      --arg path "$FILE_PATH" \
      '{"source": $source, "filePath": $path}')
    ;;

  instructions_loaded)
    FILE_PATH=$(echo "$INPUT" | jq -r '.file_path // empty' 2>/dev/null)
    LOAD_REASON=$(echo "$INPUT" | jq -r '.load_reason // empty' 2>/dev/null)
    EVENT_TYPE="instructions_loaded"
    DATA=$(jq -cn \
      --arg path "$FILE_PATH" \
      --arg reason "$LOAD_REASON" \
      '{"filePath": $path, "loadReason": $reason}')
    ;;

  task_completed)
    TASK_ID=$(echo "$INPUT" | jq -r '.task_id // empty' 2>/dev/null)
    TASK_SUBJECT=$(echo "$INPUT" | jq -r '.task_subject // empty' 2>/dev/null)
    EVENT_TYPE="task_completed"
    DATA=$(jq -cn \
      --arg id "$TASK_ID" \
      --arg subject "$TASK_SUBJECT" \
      '{"taskId": $id, "taskSubject": $subject}')
    ;;

  elicitation)
    MCP_SERVER=$(echo "$INPUT" | jq -r '.mcp_server_name // empty' 2>/dev/null)
    EVENT_TYPE="elicitation"
    DATA=$(jq -cn --arg server "$MCP_SERVER" '{"mcpServerName": $server}')
    ;;

  elicitation_result)
    MCP_SERVER=$(echo "$INPUT" | jq -r '.mcp_server_name // empty' 2>/dev/null)
    ACTION=$(echo "$INPUT" | jq -r '.action // empty' 2>/dev/null)
    EVENT_TYPE="elicitation_result"
    DATA=$(jq -cn \
      --arg server "$MCP_SERVER" \
      --arg action "$ACTION" \
      '{"mcpServerName": $server, "action": $action}')
    ;;

  stop)
    COST_DATA=$(read_cost_data)
    if [ -n "$COST_DATA" ] && [ "$COST_DATA" != "{}" ]; then
      DATA="$COST_DATA"
    fi
    ;;
esac

# -------------------------------------------------------------------
# Write event to the event directory. Atomic file write with
# nanosecond timestamp filename for rough chronological ordering.
# -------------------------------------------------------------------
JSON=$(jq -cn \
  --arg type "$EVENT_TYPE" \
  --arg cn "$CONTAINER_NAME" \
  --arg pid "$PROJECT_ID" \
  --arg wt "$WORKTREE_ID" \
  --argjson data "$DATA" \
  '{"type": $type, "containerName": $cn, "projectId": $pid, "worktreeId": $wt, "data": $data}')

warden_write_event "$JSON"

exit 0

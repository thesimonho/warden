#!/usr/bin/env bash
# -------------------------------------------------------------------
# Shared cost functions for reading Claude Code's .claude.json and
# sending cost events to the Warden event directory.
#
# Sourced by warden-event.sh, warden-capture-cost.sh,
# warden-heartbeat.sh, and warden-push-event.sh.
#
# Required variables (set by caller before sourcing):
#   WORKSPACE_DIR  — workspace directory inside the container
#   CONTAINER_NAME — container name
#   PROJECT_ID     — deterministic project identifier
#   WORKTREE_ID    — worktree identifier
#   CWD            — current working directory (optional, for exact match)
#
# Requires warden-write-event.sh to be sourced first.
# -------------------------------------------------------------------

# read_cost_data reads cost + billing type + session ID from .claude.json.
# Strategy: try exact CWD match first (most precise), then fall back
# to prefix match on WORKSPACE_DIR (handles ephemeral worktree entries
# and path variations). This mirrors the Go fallback's prefix-matching.
read_cost_data() {
  local config="/home/dev/.claude.json"
  if [ ! -f "$config" ]; then
    echo "{}"
    return
  fi
  jq -c --arg cwd "${CWD:-}" --arg prefix "$WORKSPACE_DIR" '
    ((.oauthAccount.billingType // "") == "stripe_subscription") as $isEst |
    if ($cwd != "") and .projects[$cwd].lastCost then
      {totalCost: .projects[$cwd].lastCost, messageCount: 1,
       sessionId: (.projects[$cwd].lastSessionId // ""), isEstimated: $isEst}
    else
      [.projects | to_entries[] | select(.key | startswith($prefix))]
      | if length > 0 then
          {totalCost: ([.[].value.lastCost // 0] | add), messageCount: 1,
           sessionId: (.[0].value.lastSessionId // ""), isEstimated: $isEst}
        else {totalCost: 0, messageCount: 0, sessionId: "", isEstimated: $isEst}
        end
    end
  ' "$config" 2>/dev/null || echo "{}"
}

# send_cost_event writes a stop event with cost data to the event directory.
# Used by both stop and session_end hooks, and by the post-exit capture.
send_cost_event() {
  local cost_data
  cost_data=$(read_cost_data)
  if [ -n "$cost_data" ] && [ "$cost_data" != "{}" ]; then
    warden_write_event "$(warden_build_event_json "stop" "$cost_data")"
  fi
}

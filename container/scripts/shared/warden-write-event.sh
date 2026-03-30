#!/usr/bin/env bash
# -------------------------------------------------------------------
# Shared atomic file write function for Warden event delivery.
#
# Sourced by all event-producing scripts. Writes a JSON event file
# atomically to the bind-mounted event directory using write-to-tmp
# then rename, so the host-side watcher never reads a partial file.
#
# Environment:
#   WARDEN_EVENT_DIR — path to the event directory inside the container
#                      (default: /var/warden/events)
#
# Safety valve: if the event directory has more than 50,000 files,
# the oldest files are removed before writing. Checked at most once
# per minute to avoid per-event subprocess overhead.
# -------------------------------------------------------------------

# Maximum number of event files before dropping oldest.
_WARDEN_MAX_EVENT_FILES=50000

# Throttle: only check safety valve once per minute.
_WARDEN_LAST_VALVE_CHECK=0

# _warden_enforce_safety_valve removes the oldest event files when the
# directory exceeds the limit. Throttled to once per minute to avoid
# spawning find/wc subprocesses on every write.
_warden_enforce_safety_valve() {
  local now
  now=$(date +%s)
  if (( now - _WARDEN_LAST_VALVE_CHECK < 60 )); then
    return
  fi
  _WARDEN_LAST_VALVE_CHECK=$now

  local count
  count=$(find "${WARDEN_EVENT_DIR}" -maxdepth 1 -name '*.json' -type f 2>/dev/null | wc -l)
  if [ "$count" -gt "$_WARDEN_MAX_EVENT_FILES" ]; then
    local excess=$(( count - _WARDEN_MAX_EVENT_FILES + 1000 ))
    find "${WARDEN_EVENT_DIR}" -maxdepth 1 -name '*.json' -type f 2>/dev/null \
      | sort | head -n "$excess" | xargs rm -f 2>/dev/null || true
  fi
}

# warden_check_event_env validates that the required environment
# variables for event delivery are set. Returns 1 if any are missing,
# allowing callers to exit early: `warden_check_event_env || exit 0`
#
# Sets CONTAINER_NAME and PROJECT_ID as side effects (shared by all
# event scripts).
warden_check_event_env() {
  CONTAINER_NAME="${WARDEN_CONTAINER_NAME:-}"
  if [ -z "$CONTAINER_NAME" ]; then return 1; fi
  PROJECT_ID="${WARDEN_PROJECT_ID:-}"
  if [ -z "${WARDEN_EVENT_DIR:-}" ]; then return 1; fi
  return 0
}

# warden_extract_worktree_id derives the worktree ID from a cwd path.
# Claude Code worktrees: .claude/worktrees/<id>
# Legacy worktrees: .worktrees/<id>
# Workspace root: "main"
#
# Usage: WORKTREE_ID=$(warden_extract_worktree_id "$CWD" "$WORKSPACE_DIR")
warden_extract_worktree_id() {
  local cwd="$1" workspace_dir="$2"
  if [ -z "$cwd" ]; then
    return
  fi
  if [[ "$cwd" =~ /\.claude/worktrees/([^/]+) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  elif [[ "$cwd" =~ /\.worktrees/([^/]+) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  elif [ "$cwd" = "$workspace_dir" ] || [[ "$cwd" =~ ^${workspace_dir}/ ]]; then
    printf 'main'
  fi
}

# warden_extract_field extracts a simple top-level string value from
# JSON using bash pattern matching. Avoids forking jq for simple reads.
# Returns empty string if the field is missing or null.
#
# Only safe for simple identifier-like values (IDs, type strings, paths).
# For fields with arbitrary user content (tool_input, prompt text), use jq.
#
# Usage: VALUE=$(warden_extract_field "$JSON" "field_name")
warden_extract_field() {
  local input="$1" field="$2"
  if [[ "$input" =~ \"${field}\":\"([^\"]*) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  fi
}

# warden_build_event_json constructs the standard event envelope as a
# compact JSON string using bash interpolation. Avoids forking jq.
#
# All envelope fields (type, containerName, projectId, worktreeId) are
# controlled identifiers that cannot contain JSON-special characters.
# The data argument must be a valid JSON fragment (object or scalar).
#
# Requires CONTAINER_NAME, PROJECT_ID, and WORKTREE_ID to be set.
#
# Usage: JSON=$(warden_build_event_json "$event_type" "$data_json")
warden_build_event_json() {
  local event_type="$1" data="$2"
  printf '{"type":"%s","containerName":"%s","projectId":"%s","worktreeId":"%s","data":%s}' \
    "$event_type" "$CONTAINER_NAME" "$PROJECT_ID" "$WORKTREE_ID" "$data"
}

# warden_write_event writes a JSON event to the event directory.
# The file is written atomically: first to a .tmp file, then renamed
# to the final .json name. The filename uses nanosecond epoch + PID
# for rough chronological ordering and collision avoidance.
#
# A timestamp is added to the JSON payload using a single date call
# that also provides the filename epoch, avoiding redundant forks.
#
# Usage: warden_write_event "$JSON_PAYLOAD"
warden_write_event() {
  if [ -z "${WARDEN_EVENT_DIR:-}" ]; then
    return 0
  fi

  local json="$1"
  if [ -z "$json" ]; then
    return 0
  fi

  # Single date call produces both the ISO timestamp (for the payload)
  # and the epoch_ns (for the filename). Avoids forking date twice.
  local epoch_ns ts
  epoch_ns=$(date +%s%N 2>/dev/null || date +%s000000000)
  local epoch_s="${epoch_ns:0:10}"
  local nano="${epoch_ns:10}"
  ts=$(date -u -d "@${epoch_s}" +%Y-%m-%dT%H:%M:%S 2>/dev/null)
  if [ -n "$ts" ]; then
    ts="${ts}.${nano}Z"
  else
    # Fallback for systems without date -d (e.g. BusyBox).
    ts=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
  fi

  # Add timestamp to the event payload. Uses bash string manipulation
  # instead of jq to avoid a subprocess fork on every event write.
  # Safe because json is always a compact {...} object we constructed.
  if [ -n "$ts" ]; then
    json="${json%\}},\"timestamp\":\"${ts}\"}"
  fi

  _warden_enforce_safety_valve

  local filename="${epoch_ns}-$$.json"
  local tmp_path="${WARDEN_EVENT_DIR}/.${filename}.tmp"
  local final_path="${WARDEN_EVENT_DIR}/${filename}"

  printf '%s\n' "$json" > "$tmp_path" 2>/dev/null && \
    mv "$tmp_path" "$final_path" 2>/dev/null || true
}

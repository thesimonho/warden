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
  # Derive ISO timestamp from epoch_ns to avoid a second date fork.
  local epoch_s="${epoch_ns:0:10}"
  local nano="${epoch_ns:10}"
  ts=$(date -u -d "@${epoch_s}" +%Y-%m-%dT%H:%M:%S 2>/dev/null)
  if [ -n "$ts" ]; then
    ts="${ts}.${nano}Z"
  else
    # Fallback for systems without date -d (e.g. BusyBox).
    ts=$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "")
  fi

  # Add timestamp to the event payload if we got one.
  if [ -n "$ts" ]; then
    json=$(printf '%s' "$json" | jq -c --arg ts "$ts" '. + {timestamp: $ts}')
  fi

  _warden_enforce_safety_valve

  local filename="${epoch_ns}-$$.json"
  local tmp_path="${WARDEN_EVENT_DIR}/.${filename}.tmp"
  local final_path="${WARDEN_EVENT_DIR}/${filename}"

  printf '%s\n' "$json" > "$tmp_path" 2>/dev/null && \
    mv "$tmp_path" "$final_path" 2>/dev/null || true
}

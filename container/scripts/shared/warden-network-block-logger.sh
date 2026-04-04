#!/usr/bin/env bash
# -------------------------------------------------------------------
# Warden network block logger — periodically polls xt_recent for
# blocked destination IPs and writes network_blocked events so users
# can see which domains to add to their allow list.
#
# Reads /proc/net/xt_recent/warden_blocked (populated by iptables
# rules in setup-network-isolation.sh via -m recent --rdest --set).
# Resolves IPs to hostnames via reverse DNS and writes events to
# the bind-mounted event directory.
#
# Started as a background process from user-entrypoint.sh when the
# network mode is not "full". Exits silently if xt_recent is not
# available (kernel module not loaded or rule creation failed).
#
# Environment:
#   WARDEN_NETWORK_MODE   — network mode (exits if "full")
#   WARDEN_CONTAINER_NAME — container name (set by Warden)
#   WARDEN_PROJECT_ID     — project identifier (set by Warden)
#   WARDEN_EVENT_DIR      — event directory (set by Warden)
# -------------------------------------------------------------------
set -euo pipefail

INTERVAL=30
RECENT_FILE="/proc/net/xt_recent/warden_blocked"

MODE="${WARDEN_NETWORK_MODE:-full}"
if [ "$MODE" = "full" ]; then
  exit 0
fi

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

warden_check_event_env || exit 0

# Wait for xt_recent proc file to appear (iptables rules may not
# have been applied yet during container startup).
WAIT_TIMEOUT=60
WAITED=0
while [ ! -f "$RECENT_FILE" ]; do
  sleep 2
  WAITED=$((WAITED + 2))
  if [ "$WAITED" -ge "$WAIT_TIMEOUT" ]; then
    # xt_recent not available — exit silently.
    exit 0
  fi
done

# Resolve an IP to a hostname via reverse DNS. Uses getent (always
# available via glibc) which respects nsswitch.conf resolution order.
resolve_hostname() {
  local ip="$1"
  getent hosts "$ip" 2>/dev/null | awk '{print $2}' | head -1 || true
}

# Track which IPs we have already reported to avoid duplicate events.
declare -A REPORTED_IPS
# Container-level event — no specific worktree.
WORKTREE_ID=""

while true; do
  [ -f "$RECENT_FILE" ] || { sleep "$INTERVAL"; continue; }

  # xt_recent always labels the tracked address as "src=" even when
  # recorded via --rdest. With our rules, this is the blocked
  # destination IP.
  while IFS= read -r line; do
    if [[ "$line" =~ src=([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+) ]]; then
      ip="${BASH_REMATCH[1]}"
    else
      continue
    fi

    # Skip already-reported IPs.
    [ -z "${REPORTED_IPS[$ip]+x}" ] || continue
    REPORTED_IPS["$ip"]=1

    hostname=$(resolve_hostname "$ip")

    # Use jq for safe JSON construction (hostname comes from DNS).
    data=$(jq -nc --arg ip "$ip" --arg host "$hostname" \
      'if $host == "" then {ip: $ip} else {ip: $ip, hostname: $host} end')
    warden_write_event "$(warden_build_event_json "network_blocked" "$data")"
  done < "$RECENT_FILE"

  sleep "$INTERVAL"
done

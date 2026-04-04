#!/usr/bin/env bash
# -------------------------------------------------------------------
# Warden network block logger — periodically polls xt_recent for
# blocked destination IPs and writes network_blocked events so users
# can see which domains to add to their allow list.
#
# Reads /proc/net/xt_recent/warden_blocked (populated by iptables
# rules in setup-network-isolation.sh via -m recent --rdest --set).
# Resolves IPs to domain names using the dnsmasq query log (which
# records every DNS reply with IP→domain mappings). Falls back to
# reverse DNS when dnsmasq is not running (none mode).
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
DNSMASQ_LOG="/var/log/dnsmasq.log"

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

# Skip private/internal IPs — they're infrastructure addresses (Docker
# gateway, bridge network) that users can't add to a domain allow list.
is_private_ip() {
  local ip="$1"
  case "$ip" in
    10.*|172.1[6-9].*|172.2[0-9].*|172.3[0-1].*|192.168.*|169.254.*) return 0 ;;
    *) return 1 ;;
  esac
}

# Resolve an IP to a domain. Checks the dnsmasq-derived DNS_MAP first,
# falls back to reverse DNS (useful in none mode where dnsmasq is
# not running).
resolve_domain() {
  local ip="$1"
  if [ -n "${DNS_MAP[$ip]+x}" ]; then
    printf '%s' "${DNS_MAP[$ip]}"
    return
  fi
  getent hosts "$ip" 2>/dev/null | awk '{print $2}' | head -1 || true
}

# Track which IPs we have already reported to avoid duplicate events.
declare -A REPORTED_IPS
declare -A DNS_MAP
# Container-level event — no specific worktree.
WORKTREE_ID=""

while true; do
  [ -f "$RECENT_FILE" ] || { sleep "$INTERVAL"; continue; }

  # Refresh IP→domain mapping from dnsmasq log before processing.
  # Inline rather than a function to avoid declare -gA resetting the array.
  if [ -f "$DNSMASQ_LOG" ]; then
    while read -r _ip _domain; do
      if [ -n "$_ip" ]; then DNS_MAP["$_ip"]="$_domain"; fi
    done < <(awk '/reply .* is [0-9]+\./{
      for(i=1;i<=NF;i++){
        if($i=="reply"){domain=$(i+1)}
        if($i=="is" && $(i+1)~/^[0-9]+\./){print $(i+1), domain}
      }
    }' "$DNSMASQ_LOG")
  fi

  # xt_recent always labels the tracked address as "src=" even when
  # recorded via --rdest. With our rules, this is the blocked
  # destination IP.
  while IFS= read -r line; do
    if [[ "$line" =~ src=([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+) ]]; then
      ip="${BASH_REMATCH[1]}"
    else
      continue
    fi

    # Skip private/internal and already-reported IPs.
    if is_private_ip "$ip"; then continue; fi
    if [ -n "${REPORTED_IPS[$ip]+x}" ]; then continue; fi
    REPORTED_IPS["$ip"]=1

    domain=$(resolve_domain "$ip")

    # Use jq for safe JSON construction (domain comes from DNS).
    data=$(jq -nc --arg ip "$ip" --arg domain "$domain" \
      'if $domain == "" then {ip: $ip} else {ip: $ip, domain: $domain} end')
    warden_write_event "$(warden_build_event_json "network_blocked" "$data")"
  done < "$RECENT_FILE"

  sleep "$INTERVAL"
done

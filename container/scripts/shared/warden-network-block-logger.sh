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

# Skip private/internal IPs (RFC 1918 + link-local) — they're
# infrastructure addresses (Docker gateway, bridge network) that
# users can't add to a domain allow list.
is_private_ip() {
  local ip="$1" octet1 octet2
  octet1="${ip%%.*}"
  octet2="${ip#*.}"; octet2="${octet2%%.*}"
  case "$octet1" in
    10|127) return 0 ;;
    172) [ "$octet2" -ge 16 ] && [ "$octet2" -le 31 ] && return 0 ;;
    192) [ "$octet2" -eq 168 ] && return 0 ;;
    169) [ "$octet2" -eq 254 ] && return 0 ;;
  esac
  return 1
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

# Load allowed domains from dnsmasq config for filtering false positives.
# When an allowed domain's ipset entry expires (300s TTL) and something
# reconnects using a cached IP, iptables blocks it briefly until the
# next DNS lookup refreshes the ipset. These are transient, not real blocks.
declare -a ALLOWED_DOMAINS=()
if [ -f /etc/dnsmasq.d/warden.conf ]; then
  while read -r d; do
    if [ -n "$d" ]; then ALLOWED_DOMAINS+=("$d"); fi
  done < <(awk -F'[=/]' '/^ipset=/{print $2}' /etc/dnsmasq.d/warden.conf)
fi

# Check if a domain is covered by the allowed domains list.
is_allowed_domain() {
  local domain="$1"
  if [ -z "$domain" ]; then return 1; fi
  for allowed in "${ALLOWED_DOMAINS[@]}"; do
    if [ "$domain" = "$allowed" ] || [[ "$domain" == *."$allowed" ]]; then
      return 0
    fi
  done
  return 1
}

# Track which IPs we have already reported to avoid duplicate events.
declare -A REPORTED_IPS
declare -A DNS_MAP
# Track retry attempts for IPs with unresolved domains.
declare -A RETRY_COUNT
# Container-level event — no specific worktree.
WORKTREE_ID=""
# Byte offset for incremental dnsmasq log parsing.
LOG_OFFSET=0

while true; do
  [ -f "$RECENT_FILE" ] || { sleep "$INTERVAL"; continue; }

  # Collect new (unreported) IPs from xt_recent before doing any work.
  declare -a NEW_IPS=()
  while IFS= read -r line; do
    if [[ "$line" =~ src=([0-9]+\.[0-9]+\.[0-9]+\.[0-9]+) ]]; then
      ip="${BASH_REMATCH[1]}"
      if is_private_ip "$ip"; then continue; fi
      if [ -n "${REPORTED_IPS[$ip]+x}" ]; then continue; fi
      NEW_IPS+=("$ip")
    fi
  done < "$RECENT_FILE"

  # Only parse the dnsmasq log if there are new IPs to resolve.
  if [ ${#NEW_IPS[@]} -gt 0 ] && [ -f "$DNSMASQ_LOG" ]; then
    # Incremental parse — only read new bytes since last poll.
    current_size=$(wc -c < "$DNSMASQ_LOG" 2>/dev/null || echo 0)
    if [ "$current_size" -gt "$LOG_OFFSET" ]; then
      while read -r _ip _domain; do
        if [ -n "$_ip" ]; then DNS_MAP["$_ip"]="$_domain"; fi
      done < <(tail -c +"$((LOG_OFFSET + 1))" "$DNSMASQ_LOG" | awk '
        # Forward DNS replies: "reply example.com is 93.184.216.34"
        /reply .* is [0-9]+\./{
          for(i=1;i<=NF;i++){
            if($i=="reply"){domain=$(i+1)}
            if($i=="is" && $(i+1)~/^[0-9]+\./){print $(i+1), domain}
          }
        }
        # ipset additions: "ipset add warden_allowed 1.2.3.4 github.com"
        # Covers IPs resolved by dnsmasq for allowed domains. When the
        # ipset TTL expires and the connection gets transiently blocked,
        # this mapping lets the false-positive filter skip the event.
        /ipset add warden_allowed [0-9]+\./{
          for(i=1;i<=NF;i++){
            if($i=="warden_allowed" && $(i+1)~/^[0-9]+\./){print $(i+1), $(i+2)}
          }
        }
      ')
      LOG_OFFSET=$current_size
    fi
  fi

  # Process new blocked IPs. If domain resolution fails (dnsmasq log
  # may not have the DNS reply yet), defer the IP to the next poll
  # cycle instead of reporting without a domain.
  for ip in "${NEW_IPS[@]}"; do
    domain=$(resolve_domain "$ip")

    # Skip IPs that belong to allowed domains — these are transient
    # blocks from expired ipset entries, not real policy violations.
    if is_allowed_domain "$domain"; then
      REPORTED_IPS["$ip"]=1
      continue
    fi

    # Defer IPs with no resolved domain — the dnsmasq log may not
    # have the DNS reply yet. Retry on the next poll cycle.
    if [ -z "$domain" ] && [ -f "$DNSMASQ_LOG" ] && [ "${RETRY_COUNT[$ip]:-0}" -lt 3 ]; then
      RETRY_COUNT["$ip"]=$(( ${RETRY_COUNT[$ip]:-0} + 1 ))
      continue
    fi

    REPORTED_IPS["$ip"]=1

    # Use jq for safe JSON construction (domain comes from DNS).
    data=$(jq -nc --arg ip "$ip" --arg domain "$domain" \
      'if $domain == "" then {ip: $ip} else {ip: $ip, domain: $domain} end')
    warden_write_event "$(warden_build_event_json "network_blocked" "$data")"
  done
  unset NEW_IPS

  sleep "$INTERVAL"
done

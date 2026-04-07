#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Network isolation via iptables OUTPUT rules.
#
# Reads WARDEN_NETWORK_MODE and WARDEN_ALLOWED_DOMAINS env vars to
# configure outbound firewall rules. Runs as root in the entrypoint
# before any user code executes, and can be re-run via docker exec
# to hot-reload allowed domains without recreating the container.
# After first-run setup, a smoke test verifies that blocked IPs are
# unreachable (TCP probe) and allowed domains resolve via dnsmasq.
#
# Modes:
#   "none"       — block all outbound traffic (air-gapped)
#   "restricted" — dnsmasq + ipset for dynamic domain-based filtering
#   "full"       — no restrictions (this script exits immediately)
#
# Restricted mode uses dnsmasq as a local DNS forwarder. When a DNS
# query matches an allowed domain, dnsmasq adds the resolved IPs to
# an ipset. iptables allows traffic to any IP in the set. This handles
# wildcard domains correctly — e.g. *.github.com covers ssh.github.com
# even when it resolves to different IPs than github.com.
#
# Hot-reload: when re-run on a container where dnsmasq is already
# running, the script regenerates the dnsmasq config, flushes and
# re-seeds the ipset, and signals dnsmasq to reload — without
# touching iptables rules or resolv.conf.
#
# Requires NET_ADMIN capability (added by the Go container creation
# code for non-full modes).
# -------------------------------------------------------------------

MODE="${WARDEN_NETWORK_MODE:-full}"

if [ "$MODE" = "full" ]; then
  exit 0
fi

# --- Shared helpers ---

# Resolve a domain token to IPv4 addresses (one per line).
# Strips leading *. before resolving.
resolve_domain_ips() {
  local domain
  domain=$(echo "$1" | xargs)
  [ -z "$domain" ] && return
  domain="${domain#\*.}"
  [ -z "$domain" ] && return
  getent ahosts "$domain" 2>/dev/null \
    | awk '$1 ~ /^[0-9]+\./ {print $1}' | sort -u || true
}

# Append an iptables REJECT rule with xt_recent tracking for blocked
# destination IPs. Falls back to a plain REJECT if xt_recent is not
# available (kernel module not loaded).
reject_and_track() {
  iptables -A OUTPUT -m recent --name warden_blocked --rdest --set \
    -j REJECT --reject-with icmp-port-unreachable 2>/dev/null \
    || iptables -A OUTPUT -j REJECT --reject-with icmp-port-unreachable
}

# Verify the firewall is working by checking that a known-blocked
# domain is unreachable and at least one allowed domain resolves.
# Warns on failure but does not abort — a false negative from a
# transient DNS issue is worse than a missing verification.
verify_firewall() {
  local blocked_ok=false allowed_ok=false

  # Check that a known-blocked IP is unreachable at the transport layer.
  # 93.184.216.34 is example.com's well-known stable IPv4 address.
  # Uses /dev/tcp for a direct TCP probe — tests iptables, not DNS.
  # REJECT gives instant ECONNREFUSED; timeout catches DROP rules.
  if ! timeout 2 bash -c 'echo > /dev/tcp/93.184.216.34/80' 2>/dev/null; then
    blocked_ok=true
  fi

  # Skip allowed-domain check if no domains configured.
  if [ -z "${ALLOWED_DOMAINS:-}" ]; then
    if [ "$blocked_ok" = true ]; then
      echo "[warden] firewall verification passed (no allowed domains)"
    else
      echo "[warden] warning: firewall verification failed — blocked IP (93.184.216.34) was reachable" >&2
    fi
    return
  fi

  # Check that at least one allowed domain resolves via dnsmasq.
  # Retries handle dnsmasq warmup delay after resolv.conf rewrite.
  IFS=',' read -ra _vdomains <<< "$ALLOWED_DOMAINS"
  for d in "${_vdomains[@]}"; do
    d=$(echo "$d" | xargs)
    d="${d#\*.}"
    [ -z "$d" ] && continue
    for _attempt in 1 2 3; do
      if getent ahosts "$d" >/dev/null 2>&1; then
        allowed_ok=true
        break 2
      fi
      sleep 0.5
    done
  done

  if [ "$blocked_ok" = true ] && [ "$allowed_ok" = true ]; then
    echo "[warden] firewall verification passed"
  elif [ "$blocked_ok" = false ]; then
    echo "[warden] warning: firewall verification failed — blocked IP (93.184.216.34) was reachable" >&2
  elif [ "$allowed_ok" = false ]; then
    echo "[warden] warning: firewall verification failed — no allowed domain could be resolved" >&2
  fi
}

# Detect hot-reload: if dnsmasq is already running, this is a re-run
# via docker exec. Skip iptables setup (rules are already in place)
# and jump straight to domain config update.
IS_RELOAD=false
if pgrep -x dnsmasq >/dev/null 2>&1; then
  IS_RELOAD=true
fi

if [ "$IS_RELOAD" = "false" ]; then
  # Flush any existing OUTPUT rules to start clean.
  iptables -F OUTPUT 2>/dev/null || true

  # Always allow loopback (required for internal services and dnsmasq).
  iptables -A OUTPUT -o lo -j ACCEPT

  # Allow established/related connections (responses to incoming
  # traffic from the host).
  iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT
fi

if [ "$MODE" = "none" ]; then
  # Air-gapped: reject everything else (REJECT gives instant failure vs DROP's 5min timeout).
  reject_and_track
  echo "[warden] network isolation: air-gapped (all outbound blocked)"
  exit 0
fi

if [ "$MODE" = "restricted" ]; then
  ALLOWED_DOMAINS="${WARDEN_ALLOWED_DOMAINS:-}"

  if [ -z "$ALLOWED_DOMAINS" ]; then
    echo "[warden] network isolation: restricted but no domains allowed"
    if [ "$IS_RELOAD" = "false" ]; then
      reject_and_track
    fi
    exit 0
  fi

  if [ "$IS_RELOAD" = "false" ]; then
    # --- Capture upstream DNS before any changes ---
    # On reload, resolv.conf already points to dnsmasq (127.0.0.53),
    # so upstream DNS capture only works on first run.
    UPSTREAM_DNS=$(awk '/^nameserver/{print $2}' /etc/resolv.conf | sort -u)

    # Allow DNS to upstream servers (so dnsmasq can forward queries).
    for dns in $UPSTREAM_DNS; do
      iptables -A OUTPUT -p udp -d "$dns" --dport 53 -j ACCEPT
      iptables -A OUTPUT -p tcp -d "$dns" --dport 53 -j ACCEPT
    done
  fi

  # On reload, recover upstream DNS from the existing dnsmasq config
  # since resolv.conf now points to dnsmasq itself.
  if [ "$IS_RELOAD" = "true" ] && [ -f /etc/dnsmasq.d/warden.conf ]; then
    UPSTREAM_DNS=$(awk -F= '/^server=/{print $2}' /etc/dnsmasq.d/warden.conf | sort -u)
  fi

  # --- Check if dnsmasq + ipset are available ---
  # Both are installed by install-tools.sh, but custom images may lack them.
  if ! command -v dnsmasq >/dev/null 2>&1 || ! command -v ipset >/dev/null 2>&1; then
    echo "[warden] warning: dnsmasq or ipset not available, falling back to static resolution"
    IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
    for domain in "${DOMAINS[@]}"; do
      for ip in $(resolve_domain_ips "$domain"); do
        iptables -A OUTPUT -d "$ip" -j ACCEPT
      done
    done
    reject_and_track
    echo "[warden] network isolation: restricted/static ($(echo "$ALLOWED_DOMAINS" | tr ',' ' '))"
    exit 0
  fi

  # --- Create ipset with auto-expiry ---
  # Entries expire after 300s unless refreshed by a new DNS lookup.
  # The ESTABLISHED,RELATED rule keeps existing connections alive.
  ipset create warden_allowed hash:ip timeout 300 2>/dev/null || true

  # --- Generate dnsmasq config ---
  # dnsmasq's /domain/ syntax matches the domain AND all subdomains,
  # so *.github.com is written as /github.com/ (strip *. prefix).
  mkdir -p /etc/dnsmasq.d
  {
    echo "# Warden network isolation — generated by setup-network-isolation.sh"
    echo "listen-address=127.0.0.53"
    echo "bind-interfaces"
    echo "no-resolv"
    echo "cache-size=1000"
    echo "log-queries"
    echo "log-facility=/var/log/dnsmasq.log"
    for dns in $UPSTREAM_DNS; do
      echo "server=$dns"
    done
    IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
    for domain in "${DOMAINS[@]}"; do
      domain=$(echo "$domain" | xargs)
      [ -z "$domain" ] && continue
      domain="${domain#\*.}"
      [ -z "$domain" ] && continue
      echo "ipset=/${domain}/warden_allowed"
    done
  } > /etc/dnsmasq.d/warden.conf

  # --- Flush and re-seed ipset ---
  # Flush removes stale entries from previous domains. Re-seed with
  # upstream DNS to avoid a bootstrap race where a process connects
  # before dnsmasq has populated the set.
  ipset flush warden_allowed 2>/dev/null || true
  IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
  for domain in "${DOMAINS[@]}"; do
    for ip in $(resolve_domain_ips "$domain"); do
      ipset add warden_allowed "$ip" timeout 300 2>/dev/null || true
    done
  done

  # --- Hot-reload: signal dnsmasq and exit ---
  # Config and ipset are already updated above. SIGHUP causes dnsmasq
  # to re-read its config files. No need to touch iptables or resolv.conf
  # since those are unchanged from the initial run.
  if [ "$IS_RELOAD" = "true" ]; then
    pkill -HUP -x dnsmasq 2>/dev/null || true
    echo "[warden] network isolation: reloaded ($(echo "$ALLOWED_DOMAINS" | tr ',' ' '))"
    exit 0
  fi

  # --- First run: set up iptables rules ---
  iptables -A OUTPUT -m set --match-set warden_allowed dst -j ACCEPT

  # Reject everything else (instant failure vs DROP's silent 5min timeout).
  reject_and_track

  # --- Start dnsmasq and wait for it to be ready ---
  dnsmasq --conf-dir=/etc/dnsmasq.d --keep-in-foreground &
  DNSMASQ_PID=$!

  for _ in $(seq 1 20); do
    if ss -lun sport = 53 2>/dev/null | grep -q 127.0.0.53; then
      break
    fi
    sleep 0.1
  done

  if ! kill -0 "$DNSMASQ_PID" 2>/dev/null; then
    echo "[warden] error: dnsmasq failed to start" >&2
    exit 1
  fi

  # Make the log file readable by the warden user so the block logger
  # can parse DNS replies. Runs after confirming dnsmasq is alive so
  # the file exists.
  chmod 644 /var/log/dnsmasq.log 2>/dev/null || true

  # --- Rewrite resolv.conf to route DNS through dnsmasq ---
  if ! echo "nameserver 127.0.0.53" > /etc/resolv.conf 2>/dev/null; then
    echo "[warden] error: could not rewrite resolv.conf for dnsmasq" >&2
    exit 1
  fi

  echo "[warden] network isolation: restricted/dynamic via dnsmasq ($(echo "$ALLOWED_DOMAINS" | tr ',' ' '))"
  # Smoke test runs only on first run — the IS_RELOAD branch exits above.
  verify_firewall
  exit 0
fi

echo "[warden] warning: unknown network mode: $MODE"

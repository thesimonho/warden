#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Network isolation via iptables OUTPUT rules.
#
# Reads WARDEN_NETWORK_MODE and WARDEN_ALLOWED_DOMAINS env vars to
# configure outbound firewall rules. Runs as root in the entrypoint
# before any user code executes.
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
# Requires NET_ADMIN capability (added by the Go container creation
# code for non-full modes).
# -------------------------------------------------------------------

MODE="${WARDEN_NETWORK_MODE:-full}"

if [ "$MODE" = "full" ]; then
  exit 0
fi

# Flush any existing OUTPUT rules to start clean.
iptables -F OUTPUT 2>/dev/null || true

# Always allow loopback (required for internal services and dnsmasq).
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established/related connections (responses to incoming
# traffic from the host).
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

if [ "$MODE" = "none" ]; then
  # Air-gapped: drop everything else.
  iptables -A OUTPUT -j DROP
  echo "[warden] network isolation: air-gapped (all outbound blocked)"
  exit 0
fi

if [ "$MODE" = "restricted" ]; then
  # --- Capture upstream DNS before any changes ---
  UPSTREAM_DNS=$(awk '/^nameserver/{print $2}' /etc/resolv.conf | sort -u)

  # Allow DNS to upstream servers (so dnsmasq can forward queries).
  for dns in $UPSTREAM_DNS; do
    iptables -A OUTPUT -p udp -d "$dns" --dport 53 -j ACCEPT
    iptables -A OUTPUT -p tcp -d "$dns" --dport 53 -j ACCEPT
  done

  ALLOWED_DOMAINS="${WARDEN_ALLOWED_DOMAINS:-}"

  if [ -z "$ALLOWED_DOMAINS" ]; then
    echo "[warden] network isolation: restricted but no domains allowed"
    iptables -A OUTPUT -j DROP
    exit 0
  fi

  # --- Check if dnsmasq + ipset are available ---
  if ! command -v dnsmasq >/dev/null 2>&1 || ! command -v ipset >/dev/null 2>&1; then
    echo "[warden] warning: dnsmasq or ipset not available, falling back to static resolution"
    # Fall back to the static resolution approach.
    IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
    for domain in "${DOMAINS[@]}"; do
      domain=$(echo "$domain" | xargs)
      [ -z "$domain" ] && continue
      resolve_domain="$domain"
      if echo "$domain" | grep -q '^\*\.'; then
        resolve_domain="${domain#\*.}"
      fi
      resolved_ips=$(getent ahosts "$resolve_domain" 2>/dev/null \
        | awk '$1 ~ /^[0-9]+\./ {print $1}' | sort -u || true)
      for ip in $resolved_ips; do
        iptables -A OUTPUT -d "$ip" -j ACCEPT
      done
    done
    iptables -A OUTPUT -j DROP
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
    echo "# Warden network isolation — generated at container start"
    echo "listen-address=127.0.0.53"
    echo "bind-interfaces"
    echo "no-resolv"
    echo "cache-size=1000"
    # Upstream DNS servers.
    for dns in $UPSTREAM_DNS; do
      echo "server=$dns"
    done
    # ipset rules for each allowed domain.
    IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
    for domain in "${DOMAINS[@]}"; do
      domain=$(echo "$domain" | xargs)
      [ -z "$domain" ] && continue
      # Strip wildcard prefix — dnsmasq matches subdomains by default.
      domain="${domain#\*.}"
      [ -z "$domain" ] && continue
      echo "ipset=/${domain}/warden_allowed"
    done
  } > /etc/dnsmasq.d/warden.conf

  # --- Seed ipset with initial resolution ---
  # Resolve each domain using the original upstream DNS and pre-populate
  # the ipset. This avoids a bootstrap race where a process makes a
  # request before dnsmasq has populated the set.
  IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
  for domain in "${DOMAINS[@]}"; do
    domain=$(echo "$domain" | xargs)
    [ -z "$domain" ] && continue
    resolve_domain="${domain#\*.}"
    [ -z "$resolve_domain" ] && continue
    resolved_ips=$(getent ahosts "$resolve_domain" 2>/dev/null \
      | awk '$1 ~ /^[0-9]+\./ {print $1}' | sort -u || true)
    for ip in $resolved_ips; do
      ipset add warden_allowed "$ip" timeout 300 2>/dev/null || true
    done
  done

  # --- iptables: allow traffic to IPs in the ipset ---
  iptables -A OUTPUT -m set --match-set warden_allowed dst -j ACCEPT

  # Drop everything else.
  iptables -A OUTPUT -j DROP

  # --- Start dnsmasq ---
  dnsmasq --conf-dir=/etc/dnsmasq.d --keep-in-foreground &
  DNSMASQ_PID=$!
  sleep 0.2

  if ! kill -0 "$DNSMASQ_PID" 2>/dev/null; then
    echo "[warden] warning: dnsmasq failed to start, DNS-based filtering unavailable"
  fi

  # --- Rewrite resolv.conf to route DNS through dnsmasq ---
  if echo "nameserver 127.0.0.53" > /etc/resolv.conf 2>/dev/null; then
    echo "[warden] network isolation: restricted/dynamic via dnsmasq ($(echo "$ALLOWED_DOMAINS" | tr ',' ' '))"
  else
    echo "[warden] warning: could not rewrite resolv.conf, DNS queries bypass dnsmasq"
    echo "[warden] network isolation: restricted/static seed only"
  fi

  exit 0
fi

echo "[warden] warning: unknown network mode: $MODE"

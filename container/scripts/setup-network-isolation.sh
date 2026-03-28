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
#   "restricted" — allow only DNS + resolved allowed domains
#   "full"       — no restrictions (this script should not be called)
#
# Requires NET_ADMIN capability (added by the Go container creation
# code for non-full modes).
#
# Limitations:
#   - Domain IPs are resolved once at container start. If a CDN
#     rotates IPs, the container may lose access. Restart to refresh.
#   - Wildcard domains (*.example.com) are supported but resolve
#     the base domain only (e.g. *.github.com resolves github.com).
#     This works well for CDNs that share IPs with the base domain
#     but may not cover all subdomains for services like AWS/GCP.
# -------------------------------------------------------------------

MODE="${WARDEN_NETWORK_MODE:-full}"

if [ "$MODE" = "full" ]; then
  exit 0
fi

# Flush any existing OUTPUT rules to start clean
iptables -F OUTPUT 2>/dev/null || true

# Always allow loopback (required for internal services)
iptables -A OUTPUT -o lo -j ACCEPT

# Allow established/related connections (responses to incoming
# traffic from the host)
iptables -A OUTPUT -m state --state ESTABLISHED,RELATED -j ACCEPT

if [ "$MODE" = "none" ]; then
  # Air-gapped: drop everything else
  iptables -A OUTPUT -j DROP
  echo "[warden] network isolation: air-gapped (all outbound blocked)"
  exit 0
fi

if [ "$MODE" = "restricted" ]; then
  # Allow DNS — read the nameserver from resolv.conf so this works
  # with both Docker (127.0.0.11) and Podman (varies by config).
  DNS_SERVERS=$(awk '/^nameserver/{print $2}' /etc/resolv.conf | sort -u)
  for dns in $DNS_SERVERS; do
    iptables -A OUTPUT -p udp -d "$dns" --dport 53 -j ACCEPT
    iptables -A OUTPUT -p tcp -d "$dns" --dport 53 -j ACCEPT
  done

  ALLOWED_DOMAINS="${WARDEN_ALLOWED_DOMAINS:-}"

  if [ -z "$ALLOWED_DOMAINS" ]; then
    echo "[warden] network isolation: restricted but no domains allowed"
    iptables -A OUTPUT -j DROP
    exit 0
  fi

  # Resolve each domain and allow its IPv4 addresses.
  # IPv6 addresses from getent ahosts are filtered out since we only
  # configure iptables (IPv4). IPv6 traffic is blocked by default.
  #
  # Wildcard domains (*.example.com) are resolved by stripping the
  # *. prefix and resolving the base domain. This covers CDNs and
  # services that share IPs with the base domain.
  IFS=',' read -ra DOMAINS <<< "$ALLOWED_DOMAINS"
  for domain in "${DOMAINS[@]}"; do
    domain=$(echo "$domain" | xargs) # trim whitespace
    if [ -z "$domain" ]; then
      continue
    fi

    # Handle wildcard domains by resolving the base domain
    resolve_domain="$domain"
    if echo "$domain" | grep -q '^\*\.'; then
      resolve_domain="${domain#\*.}"
      if [ -z "$resolve_domain" ]; then
        echo "[warden] warning: invalid wildcard domain: $domain"
        continue
      fi
    fi

    # Filter to IPv4 addresses only (dotted-decimal format)
    resolved_ips=$(getent ahosts "$resolve_domain" 2>/dev/null \
      | awk '$1 ~ /^[0-9]+\./ {print $1}' \
      | sort -u || true)
    if [ -z "$resolved_ips" ]; then
      echo "[warden] warning: could not resolve domain: $domain"
      continue
    fi

    for ip in $resolved_ips; do
      iptables -A OUTPUT -d "$ip" -j ACCEPT
      echo "[warden] allowed: $domain -> $ip"
    done
  done

  # Drop everything else
  iptables -A OUTPUT -j DROP

  echo "[warden] network isolation: restricted ($(echo "$ALLOWED_DOMAINS" | tr ',' ' '))"
  exit 0
fi

echo "[warden] warning: unknown network mode: $MODE"

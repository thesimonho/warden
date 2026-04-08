#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Root-phase entrypoint — performs privileged setup, then permanently
# drops to the warden user via gosu (exec replaces this process).
#
# This follows the standard container pattern used by official
# postgres, redis, and mongo images: start as root, do the minimum
# privileged work, then exec as the unprivileged user so PID 1 runs
# without root.
#
# Responsibilities (root only):
#   - Match warden user UID/GID to bind mount owner
#   - Set up network isolation (iptables)
#   - exec gosu to drop privileges permanently
# -------------------------------------------------------------------

WARDEN_USER="warden"

# Clear any stale network readiness marker from previous runs. The Go
# server writes this marker after applying network isolation via
# privileged docker exec. Clearing it ensures the user-entrypoint
# blocks until isolation is confirmed on every container start.
rm -f /tmp/warden-network-ready /tmp/warden-installs-done

# -------------------------------------------------------------------
# Match warden user's UID/GID to the host user that owns the project
# directory. The Go server passes these as env vars from os.Stat()
# at container creation time.
#
# When the host UID matches the image default (1000), this is a no-op.
# -------------------------------------------------------------------
HOST_UID="${WARDEN_HOST_UID:-}"
HOST_GID="${WARDEN_HOST_GID:-}"

if [ -n "$HOST_UID" ] && [ "$HOST_UID" != "0" ]; then
  CURRENT_UID=$(id -u "${WARDEN_USER}")
  if [ "$HOST_UID" != "$CURRENT_UID" ]; then
    usermod -u "$HOST_UID" "${WARDEN_USER}"
    if [ -n "$HOST_GID" ] && [ "$HOST_GID" != "0" ]; then
      groupmod -g "$HOST_GID" "${WARDEN_USER}" 2>/dev/null || true
    fi
    # Only chown the home directory itself and known runtime subdirs.
    # Image-layer dirs (.npm, .cache) already have the correct UID from
    # the build; a recursive chown of the entire tree is expensive when
    # state has accumulated across container restarts.
    chown "${WARDEN_USER}:${WARDEN_USER}" "/home/${WARDEN_USER}" 2>/dev/null || true
    chown -R "${WARDEN_USER}:${WARDEN_USER}" "/home/${WARDEN_USER}/.local" "/home/${WARDEN_USER}/.claude" 2>/dev/null || true
  fi
fi

# -------------------------------------------------------------------
# Fix ownership of directories that Docker may auto-create as root
# when setting up bind mounts (e.g. .ssh for known_hosts, config.host).
# This runs unconditionally — even when UIDs match, Docker creates
# intermediate directories as root.
# -------------------------------------------------------------------
chown -R "${WARDEN_USER}:${WARDEN_USER}" "/home/${WARDEN_USER}/.ssh" 2>/dev/null || true

# -------------------------------------------------------------------
# Remote projects: fix workspace volume ownership. Docker creates
# volume mount points as root. The warden user needs write access for
# git clone, .warden directory creation, and agent operations.
# -------------------------------------------------------------------
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"
if [ -n "${WARDEN_CLONE_URL:-}" ] && [ -d "$WORKSPACE_DIR" ]; then
  chown "${WARDEN_USER}:${WARDEN_USER}" "$WORKSPACE_DIR" 2>/dev/null || true
fi

# -------------------------------------------------------------------
# Agent CLI installation — install the selected agent's CLI from the
# cached volume. Runs before network isolation since the download
# needs unrestricted network access on first install.
# -------------------------------------------------------------------
if [ -n "${WARDEN_AGENT_TYPE:-}" ] && [ -x /usr/local/bin/install-agent.sh ]; then
  /usr/local/bin/install-agent.sh || echo "[warden] warning: agent CLI installation failed"
fi

# -------------------------------------------------------------------
# Runtime installation — install user-selected language runtimes.
# Runs as root since apt-get and system-level installs require it.
# -------------------------------------------------------------------
if [ -n "${WARDEN_ENABLED_RUNTIMES:-}" ] && [ -x /usr/local/bin/install-runtimes.sh ]; then
  /usr/local/bin/install-runtimes.sh || echo "[warden] warning: runtime installation failed"
fi

# Signal that privileged installs are complete. The Go server waits for
# this marker before applying network isolation via docker exec, so
# agent CLI and runtime downloads finish with unrestricted network access.
touch /tmp/warden-installs-done

# -------------------------------------------------------------------
# Drop privileges permanently. gosu replaces this process (exec) so
# PID 1 becomes user-entrypoint.sh running as warden. No root process
# remains in the container after this point.
# -------------------------------------------------------------------
exec gosu "${WARDEN_USER}:${WARDEN_USER}" /usr/local/bin/user-entrypoint.sh

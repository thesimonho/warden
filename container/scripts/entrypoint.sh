#!/usr/bin/env bash
set -euo pipefail

DEV_USER="dev"
IS_ROOT=$([ "$(id -u)" = "0" ] && echo true || echo false)

# -------------------------------------------------------------------
# Match dev user's UID/GID to the workspace bind mount owner so that
# bind-mounted directories (workspace, ~/.claude, etc.) are writable
# without permission issues. The image creates dev at UID 1000 which
# covers the common case; this handles hosts with a different UID.
#
# Skipped when running as non-root (e.g. Podman --userns=keep-id),
# because the host UID is already mapped into the container directly.
# -------------------------------------------------------------------
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"

if $IS_ROOT; then
  MOUNT_UID=$(stat -c '%u' "$WORKSPACE_DIR" 2>/dev/null || echo "")
  DEV_UID=$(id -u "${DEV_USER}")
  if [ -n "$MOUNT_UID" ] && [ "$MOUNT_UID" != "0" ] && [ "$MOUNT_UID" != "$DEV_UID" ]; then
    usermod -u "$MOUNT_UID" "${DEV_USER}"
    groupmod -g "$MOUNT_UID" "${DEV_USER}" 2>/dev/null || true
    chown -R "${DEV_USER}:${DEV_USER}" "/home/${DEV_USER}" 2>/dev/null || true
  fi
fi

# -------------------------------------------------------------------
# Ensure TERM is set for full color support in terminal sessions.
# Containers default to "dumb" which suppresses ANSI color output
# in Claude Code and other CLI tools.
# -------------------------------------------------------------------
export TERM="xterm-256color"

# -------------------------------------------------------------------
# Forward all env vars passed to the container into the dev user's
# shell session. su - creates a clean login environment, so vars like
# ANTHROPIC_API_KEY would otherwise be lost. We write them to a file
# that .bashrc sources on every new shell.
#
# Vars that su - sets itself are excluded to avoid conflicts.
# -------------------------------------------------------------------
{
  while IFS= read -r line; do
    key="${line%%=*}"
    value="${line#*=}"
    case "$key" in
      HOME|USER|SHELL|LOGNAME|MAIL|PATH|PWD|OLDPWD|SHLVL|_) continue ;;
    esac
    printf 'export %s=%q\n' "$key" "$value"
  done < <(printenv)
} > /home/dev/.docker_env
if $IS_ROOT; then
  chown "${DEV_USER}:${DEV_USER}" /home/dev/.docker_env
fi

# -------------------------------------------------------------------
# Git: include host gitconfig (user.name, user.email, etc.) and mark
# workspace paths as safe. The host file is mounted read-only at
# /home/dev/.gitconfig.host; we include it via git's [include] so
# the container can layer its own settings on top.
# -------------------------------------------------------------------
GITCONFIG_HOST="/home/dev/.gitconfig.host"
if [ -f "$GITCONFIG_HOST" ]; then
  if $IS_ROOT; then
    su - "${DEV_USER}" -c "git config --global include.path '${GITCONFIG_HOST}'"
  else
    git config --global include.path "${GITCONFIG_HOST}"
  fi
fi

if $IS_ROOT; then
  su - "${DEV_USER}" -c "git config --global --add safe.directory '*'"
else
  git config --global --add safe.directory '*'
fi

# -------------------------------------------------------------------
# Terminal tracking directory — ephemeral, reset on every startup.
# Each worktree with an active terminal gets a subdirectory containing
# its port number and attention state. Stale entries are harmless and
# cleared here on startup.
# -------------------------------------------------------------------
rm -rf "${WORKSPACE_DIR}/.warden-terminals"
mkdir -p "${WORKSPACE_DIR}/.warden-terminals"
if $IS_ROOT; then
  chown "${DEV_USER}:${DEV_USER}" "${WORKSPACE_DIR}/.warden-terminals"
fi

# Prevent terminal tracking from being tracked by git
if [ ! -f "${WORKSPACE_DIR}/.warden-terminals"/.gitignore ]; then
  echo '*' > "${WORKSPACE_DIR}/.warden-terminals"/.gitignore
  if $IS_ROOT; then
    chown "${DEV_USER}:${DEV_USER}" "${WORKSPACE_DIR}/.warden-terminals"/.gitignore
  fi
fi

# -------------------------------------------------------------------
# Network isolation — apply iptables rules for restricted/none modes.
# Must run after env var forwarding so WARDEN_NETWORK_MODE is available.
# -------------------------------------------------------------------
if [ -n "${WARDEN_NETWORK_MODE:-}" ] && [ "$WARDEN_NETWORK_MODE" != "full" ]; then
  if [ -x /usr/local/bin/setup-network-isolation.sh ]; then
    /usr/local/bin/setup-network-isolation.sh || echo "[warden] warning: network isolation setup failed"
  fi
fi

# -------------------------------------------------------------------
# Heartbeat — ping the host event bus periodically so the backend can
# detect container crashes when no hook fires. Runs as the dev user
# for socket access (same root/non-root branching as git config).
# -------------------------------------------------------------------
if [ -x /usr/local/bin/warden-heartbeat.sh ]; then
  if $IS_ROOT; then
    su - "${DEV_USER}" -c "nohup /usr/local/bin/warden-heartbeat.sh >/dev/null 2>&1 &"
  else
    nohup /usr/local/bin/warden-heartbeat.sh >/dev/null 2>&1 &
  fi
fi

# -------------------------------------------------------------------
# Keep container alive. Terminals are created dynamically via
# create-terminal.sh when the user connects to a worktree.
#
# On SIGTERM (container stop), forward the signal to all child processes
# so Claude and abduco can shut down gracefully. The 30s Docker
# stop timeout gives Claude time to finish writing files and save state.
# -------------------------------------------------------------------
shutdown() {
  # Send SIGTERM to all processes in the container (except PID 1)
  kill -TERM -1 2>/dev/null
  wait
  exit 0
}
trap shutdown TERM INT
while true; do sleep 86400 & wait; done

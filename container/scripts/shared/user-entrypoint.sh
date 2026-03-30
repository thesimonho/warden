#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# User-phase entrypoint — runs as the unprivileged dev user after
# the root-phase entrypoint (entrypoint.sh) drops privileges via
# gosu. PID 1 runs as dev from this point onward.
#
# Responsibilities:
#   - Forward container env vars into login shell sessions
#   - Configure git (include host gitconfig, mark safe dirs)
#   - Set up terminal tracking directory
#   - Start heartbeat for liveness detection
#   - Keep container alive and handle graceful shutdown
# -------------------------------------------------------------------

WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-/project}"

# -------------------------------------------------------------------
# Ensure TERM is set for full color support in terminal sessions.
# Containers default to "dumb" which suppresses ANSI color output
# in Claude Code and other CLI tools.
# -------------------------------------------------------------------
export TERM="xterm-256color"

# -------------------------------------------------------------------
# Forward all env vars passed to the container into the dev user's
# shell session. gosu creates a clean environment, so vars like
# ANTHROPIC_API_KEY would otherwise be lost. We write them to a file
# that .bashrc sources on every new shell.
#
# Vars that login shells set themselves are excluded to avoid conflicts.
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

# -------------------------------------------------------------------
# Git: include host gitconfig (user.name, user.email, etc.) and mark
# workspace paths as safe. The host file is mounted read-only at
# /home/dev/.gitconfig.host; we include it via git's [include] so
# the container can layer its own settings on top.
# -------------------------------------------------------------------
GITCONFIG_HOST="/home/dev/.gitconfig.host"
if [ -f "$GITCONFIG_HOST" ]; then
  git config --global include.path "${GITCONFIG_HOST}"
fi
git config --global --add safe.directory '*'

# -------------------------------------------------------------------
# SSH: copy staged host config, stripping IdentitiesOnly so the
# forwarded ssh-agent can offer keys without being blocked by
# directives that reference key files not present in the container.
# The host file is bind-mounted read-only at ~/.ssh/config.host;
# the filtered copy at ~/.ssh/config is writable by the container.
# -------------------------------------------------------------------
SSHCONFIG_HOST="$HOME/.ssh/config.host"
if [ -f "$SSHCONFIG_HOST" ]; then
  mkdir -p "$HOME/.ssh"
  # SSH has no include mechanism like git, so we copy and filter instead.
  # Case-insensitive match (/I) because OpenSSH keywords are case-insensitive.
  (umask 077; sed '/^[[:space:]]*IdentitiesOnly/Id' "$SSHCONFIG_HOST" > "$HOME/.ssh/config")
fi

# -------------------------------------------------------------------
# Terminal tracking directory — ephemeral, reset on every startup.
# Each worktree with an active terminal gets a subdirectory containing
# its port number and attention state. Stale entries are harmless and
# cleared here on startup.
# -------------------------------------------------------------------
rm -rf "${WORKSPACE_DIR}/.warden-terminals"
mkdir -p "${WORKSPACE_DIR}/.warden-terminals"
echo '*' > "${WORKSPACE_DIR}/.warden-terminals/.gitignore"

# -------------------------------------------------------------------
# Heartbeat — ping the host event bus periodically so the backend can
# detect container crashes when no hook fires.
# -------------------------------------------------------------------
if [ -x /usr/local/bin/warden-heartbeat.sh ]; then
  nohup /usr/local/bin/warden-heartbeat.sh >/dev/null 2>&1 &
fi

# -------------------------------------------------------------------
# Keep container alive. Terminals are created dynamically via
# create-terminal.sh when the user connects to a worktree.
#
# On SIGTERM (container stop), forward the signal to all child
# processes owned by dev. Docker's 30s stop timeout handles anything
# that doesn't respond (e.g. root-owned dnsmasq from restricted mode).
# -------------------------------------------------------------------
shutdown() {
  kill -TERM -1 2>/dev/null
  wait
  exit 0
}
trap shutdown TERM INT
while true; do sleep 86400 & wait; done

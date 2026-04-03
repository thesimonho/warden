#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# User-phase entrypoint — runs as the unprivileged warden user after
# the root-phase entrypoint (entrypoint.sh) drops privileges via
# gosu. PID 1 runs as warden from this point onward.
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
# Forward all env vars passed to the container into the warden user's
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
} > /home/warden/.docker_env

# -------------------------------------------------------------------
# Git: include host gitconfig (user.name, user.email, etc.) and mark
# workspace paths as safe. The host file is mounted read-only at
# /home/warden/.gitconfig.host; we include it via git's [include] so
# the container can layer its own settings on top.
# -------------------------------------------------------------------
GITCONFIG_HOST="/home/warden/.gitconfig.host"
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
  # Write to a temp file first — the .ssh directory may be root-owned when
  # Docker auto-creates it for bind-mounted files (known_hosts, config.host).
  # If the write fails (permission denied), skip silently rather than crashing.
  if (umask 077; sed '/^[[:space:]]*IdentitiesOnly/Id' "$SSHCONFIG_HOST" > "$HOME/.ssh/config") 2>/dev/null; then
    : # success
  else
    echo "[warden] warning: could not write SSH config (permission denied), skipping"
  fi
fi

# -------------------------------------------------------------------
# Dereference symlinked config files so agents can write to them.
#
# Nix Home Manager (and similar tools) create symlinks to immutable
# store paths. Inside the container, agents follow the symlink chain
# and try to write atomically to the resolved target — which fails
# because the target directory (/nix/store/...) is read-only and
# temp file creation is blocked.
#
# Fix: replace symlinks with copies of their content. The Docker
# bind mount already resolves the symlink target (verified via
# /proc/mounts), but atomic writes need a writable parent directory.
# Copying dereferences the symlink so writes go to the container's
# writable layer. Host config changes require container recreation.
# -------------------------------------------------------------------
for config_dir in /home/warden/.claude /home/warden/.codex; do
  [ -d "$config_dir" ] || continue
  find "$config_dir" -maxdepth 1 -type l 2>/dev/null | while IFS= read -r link; do
    target=$(readlink -f "$link" 2>/dev/null) || continue
    [ -f "$target" ] || continue
    cp --remove-destination "$target" "$link" 2>/dev/null || true
  done
done

# -------------------------------------------------------------------
# Terminal tracking directory — ephemeral, reset on every startup.
# Each worktree with an active terminal gets a subdirectory containing
# its port number and attention state. Stale entries are harmless and
# cleared here on startup.
# -------------------------------------------------------------------
mkdir -p "${WORKSPACE_DIR}/.warden"
echo '*' > "${WORKSPACE_DIR}/.warden/.gitignore"
# Clean up stale terminal state but preserve exit_code files so
# auto-resume can recover sessions after container restart.
# For terminal dirs that have no exit_code (agent was killed by
# container stop, not by normal exit or Stop button), write one
# so auto-resume can recover the session.
if [ -d "${WORKSPACE_DIR}/.warden/terminals" ]; then
  for d in "${WORKSPACE_DIR}/.warden/terminals"/*/; do
    [ -d "$d" ] || continue
    if [ ! -f "${d}exit_code" ]; then
      echo "137" > "${d}exit_code"
    fi
  done
  find "${WORKSPACE_DIR}/.warden/terminals" -name "inner-cmd.sh" -delete 2>/dev/null || true
  find "${WORKSPACE_DIR}/.warden/terminals" -name "port" -delete 2>/dev/null || true
fi
mkdir -p "${WORKSPACE_DIR}/.warden/terminals"

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
# processes owned by warden. Docker's 30s stop timeout handles
# anything that doesn't respond (e.g. root-owned dnsmasq from
# restricted mode).
# -------------------------------------------------------------------
shutdown() {
  kill -TERM -1 2>/dev/null
  wait
  exit 0
}
trap shutdown TERM INT
while true; do sleep 86400 & wait; done

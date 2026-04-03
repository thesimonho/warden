#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Create the non-root warden user and configure the shell environment.
#
# Prefers UID 1000 so bind-mounted host directories (like ~/.claude)
# are writable without permission issues. When UID 1000 is taken by
# a default system user (ubuntu on ubuntu:24.04), removes it and
# claims the UID. When taken by an intentionally-added user (vscode),
# creates warden without a specific UID.
#
# Also sets up the workspace directory, agent config directories, and
# .profile env forwarding for login shells.
#
# Idempotent: skips if warden user already exists.
# -------------------------------------------------------------------

if ! id -u warden >/dev/null 2>&1; then
  existing_user=$(getent passwd 1000 2>/dev/null | cut -d: -f1 || true)
  if [ -z "$existing_user" ] || [ "$existing_user" = "ubuntu" ]; then
    userdel -r ubuntu 2>/dev/null || true
    useradd -m -s /bin/bash -u 1000 warden
  else
    useradd -m -s /bin/bash warden
  fi
fi

# -------------------------------------------------------------------
# Pre-create ~/.local/bin so .profile's PATH block picks it up during
# login shells (Claude CLI installs here).
# -------------------------------------------------------------------
mkdir -p /home/warden/.local/bin

# -------------------------------------------------------------------
# Workspace directory and agent config directories.
# Both ~/.claude and ~/.codex are mandatory bind-mount targets for
# JSONL session file parsing and config passthrough.
# -------------------------------------------------------------------
mkdir -p /project /home/warden/.claude /home/warden/.codex
chown -R warden:warden /home/warden

# -------------------------------------------------------------------
# Forward container env vars into the warden user's shell session.
# entrypoint.sh writes /home/warden/.docker_env at startup; source it
# from .profile so login shells (including non-interactive ones like
# bash -lc used by tmux) pick up TERM, API keys, etc.
# -------------------------------------------------------------------
if ! grep -q '.docker_env' /home/warden/.profile 2>/dev/null; then
  echo '[ -f /home/warden/.docker_env ] && . /home/warden/.docker_env' >> /home/warden/.profile
fi

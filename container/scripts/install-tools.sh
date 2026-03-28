#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Shared installation script for Warden terminal infrastructure.
#
# Installs abduco, Claude Code CLI, creates the dev user, and copies
# terminal management scripts to /usr/local/bin/.
#
# Used by both the container Dockerfile and the devcontainer feature.
#
# Environment variables (all optional):
#   ABDUCO_VERSION  — abduco version to install (default: 0.6)
# -------------------------------------------------------------------

ABDUCO_VERSION="${ABDUCO_VERSION:-0.6}"
SCRIPTS_DIR="$(cd "$(dirname "$0")" && pwd)"

# -------------------------------------------------------------------
# System dependencies
#
# Build deps (build-essential, pkg-config, libssl-dev) are only needed
# to compile abduco. When used from the multi-stage Dockerfile, the
# pre-built binary is already at /usr/local/bin/abduco and these are
# skipped entirely. The devcontainer feature path still compiles
# inline and cleans up build deps afterward.
# -------------------------------------------------------------------
NEED_BUILD_DEPS=false
if [ ! -f /usr/local/bin/abduco ]; then
  NEED_BUILD_DEPS=true
fi

apt-get update
apt-get install -y --no-install-recommends \
  git \
  curl \
  wget \
  jq \
  ca-certificates \
  bash \
  procps \
  iproute2 \
  psmisc \
  unzip \
  tar \
  openssh-client \
  gnupg \
  iptables

if [ "$NEED_BUILD_DEPS" = true ]; then
  apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libssl-dev
fi

# -------------------------------------------------------------------
# abduco — session management with exit status tracking
# -------------------------------------------------------------------
if [ "$NEED_BUILD_DEPS" = true ]; then
  curl -fsSL -o /tmp/abduco.tar.gz \
    "https://github.com/martanne/abduco/releases/download/v${ABDUCO_VERSION}/abduco-${ABDUCO_VERSION}.tar.gz"
  tar -xzf /tmp/abduco.tar.gz -C /tmp
  make -C "/tmp/abduco-${ABDUCO_VERSION}"
  cp "/tmp/abduco-${ABDUCO_VERSION}/abduco" /usr/local/bin/abduco

  rm -rf /tmp/abduco.tar.gz "/tmp/abduco-${ABDUCO_VERSION}"

  # Remove build deps — no longer needed at runtime
  apt-get purge -y build-essential pkg-config libssl-dev
  apt-get autoremove -y
fi

# -------------------------------------------------------------------
# Non-root user — prefer UID 1000 so bind-mounted host directories
# (like ~/.claude) are writable without permission issues.
#
# When UID 1000 is taken by a default system user (ubuntu on
# ubuntu:24.04), remove it and claim the UID. When it's taken by a
# user the image creator intentionally added (e.g. vscode on
# devcontainer base images), leave it alone and create dev without
# a specific UID to avoid destroying their setup.
# -------------------------------------------------------------------
if ! id -u dev >/dev/null 2>&1; then
  existing_user=$(getent passwd 1000 2>/dev/null | cut -d: -f1 || true)
  if [ -z "$existing_user" ] || [ "$existing_user" = "ubuntu" ]; then
    userdel -r ubuntu 2>/dev/null || true
    useradd -m -s /bin/bash -u 1000 dev
  else
    useradd -m -s /bin/bash dev
  fi
fi

# -------------------------------------------------------------------
# Claude Code CLI (native build via official installer)
#
# Pre-create ~/.local/bin so .profile's PATH block picks it up during
# the login shell. Without this, the directory doesn't exist when
# .profile runs, PATH misses it, and the installer warns.
# -------------------------------------------------------------------
mkdir -p /home/dev/.local/bin
chown -R dev:dev /home/dev/.local
if ! su - dev -c "which claude" >/dev/null 2>&1; then
  su - dev -c "curl -fsSL https://claude.ai/install.sh | bash"
fi

# -------------------------------------------------------------------
# GitHub CLI
# -------------------------------------------------------------------
if [ ! -f /usr/bin/gh ]; then
  curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg \
    | dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
  echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" \
    | tee /etc/apt/sources.list.d/github-cli.list > /dev/null
  apt-get update
  apt-get install -y --no-install-recommends gh
fi

# -------------------------------------------------------------------
# Node.js LTS — needed for npx (MCP server calls, etc.)
# -------------------------------------------------------------------
if ! command -v node >/dev/null 2>&1; then
  NODE_MAJOR=24
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get install -y --no-install-recommends nodejs
fi

# -------------------------------------------------------------------
# Workspace directory and Claude config dir
# -------------------------------------------------------------------
mkdir -p /project /home/dev/.claude
chown -R dev:dev /home/dev /home/dev/.claude

# -------------------------------------------------------------------
# Forward container env vars into the dev user's shell session.
# entrypoint.sh writes /home/dev/.docker_env at startup; source it
# from .profile so login shells (including non-interactive ones like
# bash -lc used by abduco) pick up TERM, API keys, etc.
#
# .bashrc has a non-interactive guard (case $-) that would skip
# sourcing for non-interactive login shells, so .profile is the
# correct location.
# -------------------------------------------------------------------
if ! grep -q '.docker_env' /home/dev/.profile 2>/dev/null; then
  echo '[ -f /home/dev/.docker_env ] && . /home/dev/.docker_env' >> /home/dev/.profile
fi

# -------------------------------------------------------------------
# Copy terminal scripts to /usr/local/bin/
# -------------------------------------------------------------------
for script in entrypoint.sh create-terminal.sh disconnect-terminal.sh kill-worktree.sh warden-event.sh warden-write-event.sh warden-heartbeat.sh warden-push-event.sh warden-cost-lib.sh warden-capture-cost.sh setup-network-isolation.sh; do
  if [ -f "${SCRIPTS_DIR}/${script}" ]; then
    cp "${SCRIPTS_DIR}/${script}" "/usr/local/bin/${script}"
    chmod +x "/usr/local/bin/${script}"
  fi
done

# -------------------------------------------------------------------
# Claude Code managed settings — hooks for event tracking.
# Uses /etc/claude-code/ (Linux managed settings path) so hooks
# merge with user/project settings without overwriting.
#
# NOTE: WorktreeCreate is NOT hooked — it replaces Claude Code's
# default git worktree creation and doesn't work reliably inside
# abduco terminal sessions.
# -------------------------------------------------------------------
mkdir -p /etc/claude-code
cat > /etc/claude-code/managed-settings.json <<'MANAGED_EOF'
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh session_start"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh session_end"
          }
        ]
      }
    ],
    "Stop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh stop"
          }
        ]
      }
    ],
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh notification"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh user_prompt_submit"
          }
        ]
      }
    ],
    "PreToolUse": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh pre_tool_use"
          }
        ]
      }
    ],
    "PostToolUseFailure": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh post_tool_use_failure"
          }
        ]
      }
    ],
    "StopFailure": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh stop_failure"
          }
        ]
      }
    ],
    "PermissionRequest": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh permission_request"
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh subagent_start"
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh subagent_stop"
          }
        ]
      }
    ],
    "ConfigChange": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh config_change"
          }
        ]
      }
    ],
    "InstructionsLoaded": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh instructions_loaded"
          }
        ]
      }
    ],
    "TaskCompleted": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh task_completed"
          }
        ]
      }
    ],
    "Elicitation": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh elicitation"
          }
        ]
      }
    ],
    "ElicitationResult": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event.sh elicitation_result"
          }
        ]
      }
    ]
  }
}
MANAGED_EOF

# -------------------------------------------------------------------
# Clean up apt lists
# -------------------------------------------------------------------
rm -rf /var/lib/apt/lists/*

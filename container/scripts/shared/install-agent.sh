#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install the agent CLI for this container at startup.
#
# Runs in the ROOT phase of entrypoint.sh (before network isolation
# and before gosu drops to warden). Reads WARDEN_AGENT_TYPE to decide
# which CLI to install, and uses pinned version env vars for exact
# version control.
#
# CLIs are cached in the shared warden-cache volume so subsequent
# container creates with the same version are near-instant. A version
# bump (from Warden update) triggers a fresh download.
#
# Environment:
#   WARDEN_AGENT_TYPE        — "claude-code" or "codex"
#   WARDEN_CLAUDE_VERSION    — pinned Claude Code CLI version
#   WARDEN_CODEX_VERSION     — pinned Codex CLI version
#   WARDEN_EVENT_DIR         — event directory for progress notifications
#   WARDEN_CONTAINER_NAME    — container name (for event payloads)
#   WARDEN_PROJECT_ID        — project ID (for event payloads)
# -------------------------------------------------------------------

AGENT_TYPE="${WARDEN_AGENT_TYPE:-claude-code}"
CACHE_DIR="/home/warden/.cache/warden-runtimes"

# Ensure cache directory and its parent are owned by warden.
# Docker creates .cache as root when mounting the volume; the Claude
# installer writes to .cache/claude (a sibling of warden-runtimes).
mkdir -p "$CACHE_DIR"
chown warden:warden "/home/warden/.cache" "$CACHE_DIR"

# -------------------------------------------------------------------
# Progress event helpers — write agent_installing/agent_installed
# events to the event directory for frontend status updates.
# -------------------------------------------------------------------

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

push_agent_event() {
  local event_type="$1" version="$2"
  warden_check_event_env || return 0
  local WORKTREE_ID=""
  local data
  data=$(printf '{"version":"%s"}' "$version")
  warden_write_event "$(warden_build_event_json "$event_type" "$data")"
}

# -------------------------------------------------------------------
# Architecture detection
# -------------------------------------------------------------------
detect_platform() {
  local arch
  arch=$(dpkg --print-architecture 2>/dev/null || uname -m)
  case "$arch" in
    amd64|x86_64) echo "linux-x64" ;;
    arm64|aarch64) echo "linux-arm64" ;;
    *) echo "linux-x64" ;;
  esac
}

# -------------------------------------------------------------------
# Claude Code CLI — standalone binary from GCS
# -------------------------------------------------------------------
GCS_BUCKET="https://storage.googleapis.com/claude-code-dist-86c565f3-f756-42ad-8dfa-d59b1c096819/claude-code-releases"

install_claude() {
  local version="${WARDEN_CLAUDE_VERSION:-}"
  if [ -z "$version" ]; then
    echo "[warden] WARDEN_CLAUDE_VERSION not set, skipping Claude CLI install" >&2
    return 1
  fi

  local platform
  platform=$(detect_platform)
  local cache_bin="${CACHE_DIR}/claude/claude-${version}-${platform}"

  # Check if the correct version is already installed. Uses the direct
  # path because neither root nor gosu's clean env includes ~/.local/bin.
  local claude_bin="/home/warden/.local/bin/claude"
  local installed_version
  installed_version=$("$claude_bin" --version 2>/dev/null | head -1 | grep -oP '\d+\.\d+\.\d+' || echo "")
  if [ "$installed_version" = "$version" ]; then
    return 0
  fi

  # Download to cache if not already cached. Only fire SSE events
  # on a cache miss (actual download) — cache hits are fast and silent.
  local is_new_download=false
  if [ ! -f "$cache_bin" ]; then
    is_new_download=true
    push_agent_event "agent_installing" "$version"
    mkdir -p "${CACHE_DIR}/claude"
    local url="${GCS_BUCKET}/${version}/${platform}/claude"
    echo "[warden] Downloading Claude Code ${version} (${platform})..."
    curl -fsSL "$url" -o "$cache_bin" || {
      echo "[warden] ERROR: Failed to download Claude Code ${version}" >&2
      rm -f "$cache_bin"
      return 1
    }
    chmod +x "$cache_bin"
    chown warden:warden "$cache_bin"
  fi

  # Run the installer as the warden user. Pass the version so it uses
  # the local binary instead of trying to download "latest". Include
  # ~/.local/bin in PATH so the installer's post-install PATH check
  # passes (gosu's clean env doesn't inherit shell profile PATH).
  PATH="/home/warden/.local/bin:$PATH" gosu warden "$cache_bin" install "$version" >/dev/null 2>&1 || {
    echo "[warden] ERROR: Claude Code install command failed" >&2
    return 1
  }

  # Write managed settings (hooks for event tracking).
  write_claude_managed_settings

  if [ "$is_new_download" = true ]; then
    push_agent_event "agent_installed" "$version"
  fi
}

# -------------------------------------------------------------------
# Claude Code managed settings — hooks for attention state and audit.
# -------------------------------------------------------------------
write_claude_managed_settings() {
  # Skip if already written (content is static, no need to rewrite).
  [ -f /etc/claude-code/managed-settings.json ] && return 0
  mkdir -p /etc/claude-code
  cat > /etc/claude-code/managed-settings.json <<'MANAGED_EOF'
{
  "env": {
    "DISABLE_AUTOUPDATER": "1"
  },
  "hooks": {
    "Notification": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh notification"
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
            "command": "/usr/local/bin/warden-event-claude.sh user_prompt_submit"
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
            "command": "/usr/local/bin/warden-event-claude.sh pre_tool_use"
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
            "command": "/usr/local/bin/warden-event-claude.sh permission_request"
          }
        ]
      }
    ],
    "ConfigChange": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh config_change"
          }
        ]
      }
    ],
    "InstructionsLoaded": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh instructions_loaded"
          }
        ]
      }
    ],
    "TaskCompleted": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh task_completed"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh session_end"
          }
        ]
      }
    ],
    "Elicitation": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh elicitation"
          }
        ]
      }
    ],
    "ElicitationResult": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh elicitation_result"
          }
        ]
      }
    ],
    "SubagentStart": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh subagent_start"
          }
        ]
      }
    ],
    "SubagentStop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/usr/local/bin/warden-event-claude.sh subagent_stop"
          }
        ]
      }
    ]
  }
}
MANAGED_EOF
}

# -------------------------------------------------------------------
# Codex CLI — npm global install
# -------------------------------------------------------------------
install_codex() {
  local version="${WARDEN_CODEX_VERSION:-}"
  if [ -z "$version" ]; then
    echo "[warden] WARDEN_CODEX_VERSION not set, skipping Codex CLI install" >&2
    return 1
  fi

  # Check if the correct version is already installed.
  local installed_version
  installed_version=$(codex --version 2>/dev/null | head -1 | grep -oP '\d+\.\d+\.\d+' || echo "")
  if [ "$installed_version" = "$version" ]; then
    return 0
  fi

  push_agent_event "agent_installing" "$version"

  # Use the cache volume for npm cache to speed up reinstalls.
  local npm_cache="${CACHE_DIR}/npm"
  mkdir -p "$npm_cache"

  echo "[warden] Installing Codex CLI ${version}..."
  npm install -g --cache "$npm_cache" "@openai/codex@${version}" 2>/dev/null || {
    echo "[warden] ERROR: Codex CLI installation failed" >&2
    return 1
  }

  if ! command -v codex >/dev/null 2>&1; then
    echo "[warden] ERROR: Codex CLI not on PATH after install" >&2
    return 1
  fi

  push_agent_event "agent_installed" "$version"
}

# -------------------------------------------------------------------
# Main — install the CLI for the configured agent type.
# -------------------------------------------------------------------
case "$AGENT_TYPE" in
  claude-code)
    install_claude || echo "[warden] warning: Claude Code CLI installation failed" >&2
    ;;
  codex)
    install_codex || echo "[warden] warning: Codex CLI installation failed" >&2
    ;;
  *)
    echo "[warden] unknown agent type: $AGENT_TYPE" >&2
    ;;
esac

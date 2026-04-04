#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install user-selected language runtimes in the container.
#
# Runs in the ROOT phase of entrypoint.sh (before gosu drops to warden).
# Reads WARDEN_ENABLED_RUNTIMES (comma-separated list of runtime IDs)
# and installs each one. Writes progress events to WARDEN_EVENT_DIR
# so the frontend can show installation status.
#
# The shared cache volume is mounted at /home/warden/.cache/warden-runtimes
# by the engine. Runtime env vars (GOMODCACHE, PIP_CACHE_DIR, etc.) point
# caches there so they persist across container recreates.
#
# Idempotent: checks for existing binaries before installing.
#
# Environment:
#   WARDEN_ENABLED_RUNTIMES  — comma-separated runtime IDs (e.g. "node,python,go")
#   WARDEN_WORKSPACE_DIR     — project workspace directory (for go.mod version detection)
#   WARDEN_EVENT_DIR         — event directory for progress notifications
#   WARDEN_CONTAINER_NAME    — container name (for event payloads)
#   WARDEN_PROJECT_ID        — project ID (for event payloads)
#   WARDEN_AGENT_TYPE        — agent type (for event payloads)
# -------------------------------------------------------------------

RUNTIMES="${WARDEN_ENABLED_RUNTIMES:-node}"
CACHE_DIR="/home/warden/.cache/warden-runtimes"
WORKSPACE_DIR="${WARDEN_WORKSPACE_DIR:-}"

# Ensure cache directory exists and is owned by warden.
mkdir -p "$CACHE_DIR"
chown warden:warden "$CACHE_DIR"

# Redirect apt's package cache to the shared volume so downloaded .deb
# files persist across container recreations. The install script runs as
# root so ownership isn't an issue.
APT_CACHE_DIR="${CACHE_DIR}/apt/archives"
mkdir -p "$APT_CACHE_DIR"
rm -rf /var/cache/apt/archives
ln -sf "$APT_CACHE_DIR" /var/cache/apt/archives

# -------------------------------------------------------------------
# Progress event helpers — write runtime_installing/runtime_installed
# events to the event directory for frontend status updates.
# Uses the shared event library for atomic writes and safety valve.
# -------------------------------------------------------------------

# shellcheck source=warden-write-event.sh
source /usr/local/bin/warden-write-event.sh

push_runtime_event() {
  local event_type="$1" runtime_id="$2" runtime_label="$3"
  warden_check_event_env || return 0
  WORKTREE_ID=""
  local data
  data=$(printf '{"runtimeId":"%s","runtimeLabel":"%s"}' "$runtime_id" "$runtime_label")
  warden_write_event "$(warden_build_event_json "$event_type" "$data")"
}

# -------------------------------------------------------------------
# Runtime installers
# -------------------------------------------------------------------

# Run apt-get update at most once per script invocation.
_APT_UPDATED=false
ensure_apt_updated() {
  if [ "$_APT_UPDATED" = false ]; then
    apt-get update -qq
    _APT_UPDATED=true
  fi
}

install_python() {
  # Check for all components — python3 may exist as a dependency
  # of other packages but pip and venv may be missing.
  if command -v python3 >/dev/null 2>&1 && python3 -m pip --version >/dev/null 2>&1 && python3 -m venv --help >/dev/null 2>&1; then
    return 0
  fi
  push_runtime_event "runtime_installing" "python" "Python"
  ensure_apt_updated
  apt-get install -y --no-install-recommends python3 python3-pip python3-venv >/dev/null 2>&1
  push_runtime_event "runtime_installed" "python" "Python"
}

install_go() {
  # Detect desired version from go.mod if available.
  local go_version="1.24.2"
  if [ -n "$WORKSPACE_DIR" ] && [ -f "${WORKSPACE_DIR}/go.mod" ]; then
    local mod_version
    mod_version=$(grep -m1 '^go ' "${WORKSPACE_DIR}/go.mod" | awk '{print $2}')
    if [ -n "$mod_version" ]; then
      if [[ "$mod_version" =~ ^[0-9]+\.[0-9]+$ ]]; then
        mod_version="${mod_version}.0"
      fi
      go_version="$mod_version"
    fi
  fi

  push_runtime_event "runtime_installing" "go" "Go ${go_version}"

  local arch
  arch=$(dpkg --print-architecture)
  case "$arch" in
    amd64) arch="amd64" ;;
    arm64) arch="arm64" ;;
    *) arch="amd64" ;;
  esac

  # Download tarball to cache volume if not already cached.
  local tarball="${CACHE_DIR}/go/downloads/go${go_version}.linux-${arch}.tar.gz"
  if [ ! -f "$tarball" ]; then
    mkdir -p "${CACHE_DIR}/go/downloads"
    local url="https://go.dev/dl/go${go_version}.linux-${arch}.tar.gz"
    curl -fsSL "$url" -o "$tarball"
  fi

  # Always install fresh from the cached tarball.
  rm -rf /usr/local/go
  tar -C /usr/local -xzf "$tarball"
  ln -sf /usr/local/go/bin/go /usr/local/bin/go
  ln -sf /usr/local/go/bin/gofmt /usr/local/bin/gofmt

  # Ensure module cache dirs exist for the warden user.
  mkdir -p "${CACHE_DIR}/go/mod" "${CACHE_DIR}/go/path"
  chown -R warden:warden "${CACHE_DIR}/go"

  push_runtime_event "runtime_installed" "go" "Go ${go_version}"
}

install_rust() {
  local cargo_home="${CACHE_DIR}/cargo"
  local rustup_home="${CACHE_DIR}/rustup"

  # If rustup is cached on the volume, just restore symlinks.
  if [ -x "${cargo_home}/bin/rustup" ]; then
    ln -sf "${cargo_home}/bin/cargo" /usr/local/bin/cargo
    ln -sf "${cargo_home}/bin/rustup" /usr/local/bin/rustup
    ln -sf "${cargo_home}/bin/rustc" /usr/local/bin/rustc
    return 0
  fi

  push_runtime_event "runtime_installing" "rust" "Rust"

  mkdir -p "${cargo_home}" "${rustup_home}"
  chown -R warden:warden "${cargo_home}" "${rustup_home}"

  # Install rustup as the warden user. Pass CARGO_HOME and RUSTUP_HOME
  # explicitly since gosu starts a clean environment.
  gosu warden env \
    CARGO_HOME="${cargo_home}" \
    RUSTUP_HOME="${rustup_home}" \
    bash -c 'curl --proto "=https" --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --no-modify-path 2>/dev/null'

  # Symlink cargo and rustup to PATH.
  ln -sf "${cargo_home}/bin/cargo" /usr/local/bin/cargo
  ln -sf "${cargo_home}/bin/rustup" /usr/local/bin/rustup
  ln -sf "${cargo_home}/bin/rustc" /usr/local/bin/rustc

  push_runtime_event "runtime_installed" "rust" "Rust"
}

install_ruby() {
  if command -v ruby >/dev/null 2>&1; then
    return 0
  fi
  push_runtime_event "runtime_installing" "ruby" "Ruby"
  ensure_apt_updated
  apt-get install -y --no-install-recommends ruby ruby-dev >/dev/null 2>&1
  gem install bundler --no-document 2>/dev/null || true

  mkdir -p "${CACHE_DIR}/gem"
  chown -R warden:warden "${CACHE_DIR}/gem"

  push_runtime_event "runtime_installed" "ruby" "Ruby"
}

install_lua() {
  if command -v lua >/dev/null 2>&1; then
    return 0
  fi
  push_runtime_event "runtime_installing" "lua" "Lua"
  ensure_apt_updated
  apt-get install -y --no-install-recommends lua5.4 liblua5.4-dev luarocks >/dev/null 2>&1

  mkdir -p "${CACHE_DIR}/luarocks"
  chown -R warden:warden "${CACHE_DIR}/luarocks"

  push_runtime_event "runtime_installed" "lua" "Lua"
}

# -------------------------------------------------------------------
# Main — iterate over enabled runtimes and install each.
# -------------------------------------------------------------------
for runtime in ${RUNTIMES//,/ }; do
  case "$runtime" in
    node)
      # Already in base image — no-op.
      ;;
    python)
      install_python || echo "[warden] warning: python install failed" >&2
      ;;
    go)
      install_go || echo "[warden] warning: go install failed" >&2
      ;;
    rust)
      install_rust || echo "[warden] warning: rust install failed" >&2
      ;;
    ruby)
      install_ruby || echo "[warden] warning: ruby install failed" >&2
      ;;
    lua)
      install_lua || echo "[warden] warning: lua install failed" >&2
      ;;
    *)
      echo "[warden] unknown runtime: $runtime" >&2
      ;;
  esac
done

# Clean up apt lists and prune outdated cached .deb files.
rm -rf /var/lib/apt/lists/* 2>/dev/null || true
apt-get autoclean -qq 2>/dev/null || true

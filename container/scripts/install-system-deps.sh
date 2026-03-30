#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install system dependencies for Warden containers.
#
# Installs: git, curl, jq, iptables, GitHub CLI, Node.js LTS, and
# optionally compiles abduco and fetches gosu (for the devcontainer
# feature path — the Dockerfile pre-builds these in the builder stage).
#
# Idempotent: checks for existing binaries before installing.
#
# Environment variables (all optional):
#   ABDUCO_VERSION — abduco version to install (default: 0.6)
#   GOSU_VERSION   — gosu version to install (default: 1.17)
# -------------------------------------------------------------------

ABDUCO_VERSION="${ABDUCO_VERSION:-0.6}"

# -------------------------------------------------------------------
# Build deps are only needed to compile abduco. When the pre-built
# binary exists (multi-stage Dockerfile), skip them entirely.
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
  iptables \
  ipset \
  dnsmasq-base

if [ "$NEED_BUILD_DEPS" = true ]; then
  apt-get install -y --no-install-recommends \
    build-essential \
    pkg-config \
    libssl-dev
fi

# -------------------------------------------------------------------
# gosu — lightweight privilege drop (setuid/setgid + exec).
# Pre-built in the multi-stage Dockerfile; installed here for the
# devcontainer feature path.
# -------------------------------------------------------------------
if [ ! -f /usr/local/bin/gosu ]; then
  GOSU_VERSION="${GOSU_VERSION:-1.17}"
  arch="$(dpkg --print-architecture)"
  curl -fsSL -o /usr/local/bin/gosu \
    "https://github.com/tianon/gosu/releases/download/${GOSU_VERSION}/gosu-${arch}"
  chmod +x /usr/local/bin/gosu
fi

# -------------------------------------------------------------------
# abduco — session management with exit status tracking.
# Pre-built in the multi-stage Dockerfile; compiled here for the
# devcontainer feature path.
# -------------------------------------------------------------------
if [ "$NEED_BUILD_DEPS" = true ]; then
  curl -fsSL -o /tmp/abduco.tar.gz \
    "https://github.com/martanne/abduco/releases/download/v${ABDUCO_VERSION}/abduco-${ABDUCO_VERSION}.tar.gz"
  tar -xzf /tmp/abduco.tar.gz -C /tmp
  make -C "/tmp/abduco-${ABDUCO_VERSION}"
  cp "/tmp/abduco-${ABDUCO_VERSION}/abduco" /usr/local/bin/abduco

  rm -rf /tmp/abduco.tar.gz "/tmp/abduco-${ABDUCO_VERSION}"
  apt-get purge -y build-essential pkg-config libssl-dev
  apt-get autoremove -y
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
# Node.js LTS — needed for npx (MCP servers) and Codex CLI.
# -------------------------------------------------------------------
if ! command -v node >/dev/null 2>&1; then
  NODE_MAJOR=24
  curl -fsSL "https://deb.nodesource.com/setup_${NODE_MAJOR}.x" | bash -
  apt-get install -y --no-install-recommends nodejs
fi

# -------------------------------------------------------------------
# Clean up apt lists to reduce layer size.
# -------------------------------------------------------------------
rm -rf /var/lib/apt/lists/*

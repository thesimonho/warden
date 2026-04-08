#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install system dependencies for Warden containers.
#
# Installs: git, curl, jq, iptables, tmux, GitHub CLI, Node.js LTS,
# and optionally fetches gosu (for the devcontainer feature path —
# the Dockerfile pre-builds gosu in the builder stage).
#
# Idempotent: checks for existing binaries before installing.
#
# Environment variables (all optional):
#   GOSU_VERSION — gosu version to install (default: 1.17)
# -------------------------------------------------------------------

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
  dnsmasq-base \
  aggregate \
  bubblewrap \
  sudo \
  tmux

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

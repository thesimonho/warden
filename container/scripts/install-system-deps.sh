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
apt-get upgrade -y
apt-get install -y --no-install-recommends \
  git \
  curl \
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
  socat \
  sudo \
  tmux

# -------------------------------------------------------------------
# gosu — lightweight privilege drop (setuid/setgid + exec).
# Pre-built in the multi-stage Dockerfile; installed here for the
# devcontainer feature path.
# -------------------------------------------------------------------
if [ ! -f /usr/local/bin/gosu ]; then
  GOSU_VERSION="${GOSU_VERSION:-1.19}"
  arch="$(dpkg --print-architecture)"
  curl -fsSL -o /usr/local/bin/gosu \
    "https://github.com/tianon/gosu/releases/download/${GOSU_VERSION}/gosu-${arch}"
  chmod +x /usr/local/bin/gosu
fi

# -------------------------------------------------------------------
# GitHub CLI — installed from official release binary (not the apt
# repo) to avoid shipping a Go stdlib compiled with Go 1.18.
# -------------------------------------------------------------------
if ! command -v gh >/dev/null 2>&1; then
  GH_VERSION="$(curl -fsSL https://api.github.com/repos/cli/cli/releases/latest | jq -r '.tag_name | ltrimstr("v")')"
  arch="$(dpkg --print-architecture)"
  curl -fsSL -o /tmp/gh.tar.gz \
    "https://github.com/cli/cli/releases/download/v${GH_VERSION}/gh_${GH_VERSION}_linux_${arch}.tar.gz"
  tar -xzf /tmp/gh.tar.gz -C /tmp
  mv "/tmp/gh_${GH_VERSION}_linux_${arch}/bin/gh" /usr/local/bin/gh
  chmod +x /usr/local/bin/gh
  rm -rf /tmp/gh.tar.gz "/tmp/gh_${GH_VERSION}_linux_${arch}"
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

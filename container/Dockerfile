# Project container image for Warden.
# Provides tmux (session persistence), terminal management scripts,
# and the install-agent.sh startup script that installs the correct
# agent CLI (Claude Code or Codex) on first run from a cached volume.
#
# Build:  docker build -t warden-project ./container
#
# Multi-stage: the builder stage fetches gosu. tmux is installed via
# apt in the runtime image. Layer order is optimized for caching:
# system deps and user setup change rarely, and Warden scripts change
# on every release (last layer, smallest invalidation).
#
# Agent CLIs are NOT baked into the image — they are installed at
# container startup by install-agent.sh using pinned versions passed
# via env vars (WARDEN_CLAUDE_VERSION / WARDEN_CODEX_VERSION). The
# warden-cache Docker volume caches downloads across container creates.

# ---- builder: fetch gosu ----
FROM ubuntu:24.04 AS builder

ARG GOSU_VERSION=1.19

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
  && rm -rf /var/lib/apt/lists/*

# gosu — lightweight setuid/setgid + exec for the entrypoint privilege drop.
RUN arch="$(dpkg --print-architecture)" \
  && curl -fsSL -o /usr/local/bin/gosu \
    "https://github.com/tianon/gosu/releases/download/${GOSU_VERSION}/gosu-${arch}" \
  && chmod +x /usr/local/bin/gosu \
  && gosu --version \
  && gosu nobody true

# ---- runtime: layered for optimal caching ----
FROM ubuntu:24.04

COPY --from=builder /usr/local/bin/gosu /usr/local/bin/gosu

# Layer 1: system deps (changes rarely — only on new apt packages)
COPY scripts/install-system-deps.sh /tmp/
RUN chmod +x /tmp/install-system-deps.sh && /tmp/install-system-deps.sh && rm /tmp/install-system-deps.sh

# Layer 2: warden user (changes rarely)
COPY scripts/install-user.sh /tmp/
RUN chmod +x /tmp/install-user.sh && /tmp/install-user.sh && rm /tmp/install-user.sh

# Layer 3: Warden scripts (changes every release — last for best caching)
# Only copy the runtime scripts and installer, not the install-*.sh files
# that already ran in earlier layers. This prevents install-script edits
# from invalidating this layer.
COPY scripts/install-warden.sh /tmp/warden-scripts/install-warden.sh
COPY scripts/shared/ /tmp/warden-scripts/shared/
COPY scripts/claude/ /tmp/warden-scripts/claude/
COPY scripts/codex/ /tmp/warden-scripts/codex/
RUN chmod +x /tmp/warden-scripts/install-warden.sh && /tmp/warden-scripts/install-warden.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

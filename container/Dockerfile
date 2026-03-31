# Project container image for Warden.
# Provides abduco (session persistence), Claude Code CLI, Codex CLI,
# and terminal management scripts. The Go backend connects to abduco
# sessions via docker exec, proxied over WebSocket to the browser.
#
# Build:  docker build -t warden-project ./container
#
# Multi-stage: build deps (build-essential, pkg-config, libssl-dev)
# stay in the builder stage. Only the compiled abduco binary and
# pre-fetched gosu are copied to the runtime image.
#
# Layer order is optimized for caching: system deps and user setup
# change rarely, CLI installs change on upstream releases, and Warden
# scripts change on every release (last layer, smallest invalidation).

# ---- builder: compile abduco from source + fetch gosu ----
FROM ubuntu:24.04 AS builder

ARG ABDUCO_VERSION=0.6
ARG GOSU_VERSION=1.17

RUN apt-get update \
  && apt-get install -y --no-install-recommends \
    curl \
    ca-certificates \
    build-essential \
    pkg-config \
    libssl-dev \
  && rm -rf /var/lib/apt/lists/*

RUN curl -fsSL -o /tmp/abduco.tar.gz \
    "https://github.com/martanne/abduco/releases/download/v${ABDUCO_VERSION}/abduco-${ABDUCO_VERSION}.tar.gz" \
  && tar -xzf /tmp/abduco.tar.gz -C /tmp \
  && make -C "/tmp/abduco-${ABDUCO_VERSION}" \
  && cp "/tmp/abduco-${ABDUCO_VERSION}/abduco" /usr/local/bin/abduco

# gosu — lightweight setuid/setgid + exec for the entrypoint privilege drop.
RUN arch="$(dpkg --print-architecture)" \
  && curl -fsSL -o /usr/local/bin/gosu \
    "https://github.com/tianon/gosu/releases/download/${GOSU_VERSION}/gosu-${arch}" \
  && chmod +x /usr/local/bin/gosu \
  && gosu --version \
  && gosu nobody true

# ---- runtime: layered for optimal caching ----
FROM ubuntu:24.04

COPY --from=builder /usr/local/bin/abduco /usr/local/bin/abduco
COPY --from=builder /usr/local/bin/gosu /usr/local/bin/gosu

# Layer 1: system deps (changes rarely — only on new apt packages)
COPY scripts/install-system-deps.sh /tmp/
RUN chmod +x /tmp/install-system-deps.sh && /tmp/install-system-deps.sh && rm /tmp/install-system-deps.sh

# Layer 2: warden user (changes rarely)
COPY scripts/install-user.sh /tmp/
RUN chmod +x /tmp/install-user.sh && /tmp/install-user.sh && rm /tmp/install-user.sh

# Layer 3: Claude CLI (changes when Claude releases update)
COPY scripts/install-claude.sh /tmp/
RUN chmod +x /tmp/install-claude.sh && /tmp/install-claude.sh && rm /tmp/install-claude.sh

# Layer 4: Codex CLI (changes when Codex releases update)
COPY scripts/install-codex.sh /tmp/
RUN chmod +x /tmp/install-codex.sh && /tmp/install-codex.sh && rm /tmp/install-codex.sh

# Layer 5: Warden scripts (changes every release — last for best caching)
# Only copy the runtime scripts and installer, not the install-*.sh files
# that already ran in earlier layers. This prevents install-script edits
# from invalidating this layer.
COPY scripts/install-warden.sh /tmp/warden-scripts/install-warden.sh
COPY scripts/shared/ /tmp/warden-scripts/shared/
COPY scripts/claude/ /tmp/warden-scripts/claude/
COPY scripts/codex/ /tmp/warden-scripts/codex/
RUN chmod +x /tmp/warden-scripts/install-warden.sh && /tmp/warden-scripts/install-warden.sh

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

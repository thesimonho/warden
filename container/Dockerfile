# Project container image for Warden.
# Provides abduco (session persistence), Claude Code CLI, and terminal
# management scripts. The Go backend connects to abduco sessions via
# docker exec, proxied over WebSocket to the browser.
#
# Build:  docker build -t claude-project-dev ./container
# Usage:  Used by scripts/dev-containers.sh
#
# Multi-stage: build deps (build-essential, pkg-config, libssl-dev) stay
# in the builder stage. Only the compiled abduco binary is copied to the
# runtime image, keeping it ~200MB smaller and free of compilers.

# ---- builder: compile abduco from source ----
FROM ubuntu:24.04 AS builder

ARG ABDUCO_VERSION=0.6

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

# ---- runtime: no build deps ----
FROM ubuntu:24.04

COPY --from=builder /usr/local/bin/abduco /usr/local/bin/abduco

COPY scripts/ /tmp/warden-scripts/
RUN chmod +x /tmp/warden-scripts/*.sh \
  && /tmp/warden-scripts/install-tools.sh \
  && rm -rf /tmp/warden-scripts

ENTRYPOINT ["/usr/local/bin/entrypoint.sh"]

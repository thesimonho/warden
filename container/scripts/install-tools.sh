#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Shared installation orchestrator for Warden terminal infrastructure.
#
# Calls the composable install sub-scripts in order. Used by the
# devcontainer feature path where all scripts are flat alongside this
# file. The Dockerfile calls each sub-script as a separate RUN
# instruction for layer caching.
#
# Environment variables (all optional):
#   ABDUCO_VERSION — abduco version to install (default: 0.6)
#   GOSU_VERSION   — gosu version to install (default: 1.17)
# -------------------------------------------------------------------

SCRIPTS_DIR="$(cd "$(dirname "$0")" && pwd)"

"${SCRIPTS_DIR}/install-system-deps.sh"
"${SCRIPTS_DIR}/install-user.sh"
"${SCRIPTS_DIR}/install-claude.sh"
"${SCRIPTS_DIR}/install-codex.sh"
"${SCRIPTS_DIR}/install-warden.sh"

#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Devcontainer feature install entry point.
#
# Maps devcontainer feature option env vars to install-tools.sh env
# vars, then delegates to the shared install script.
#
# At CI publish time, all scripts from container/scripts/ are copied
# alongside this file so install-tools.sh can find its siblings.
# -------------------------------------------------------------------

export ABDUCO_VERSION="${ABDUCOVERSION:-0.6}"
export GOSU_VERSION="${GOSUVERSION:-1.17}"

FEATURE_DIR="$(cd "$(dirname "$0")" && pwd)"
"${FEATURE_DIR}/install-tools.sh"

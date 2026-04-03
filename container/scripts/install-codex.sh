#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install OpenAI Codex CLI.
#
# Installs the CLI globally via npm. Requires Node.js to be installed
# first (handled by install-system-deps.sh).
#
# Codex does not currently support hooks (upstream gap). When hook
# support is added, this script should write ~/.codex/hooks.json with
# the notification event configuration.
#
# See:
#   https://github.com/openai/codex/issues/14813
#   https://github.com/openai/codex/issues/11808
#
# Idempotent: skips if codex is already available.
# -------------------------------------------------------------------

if ! command -v codex >/dev/null 2>&1; then
  npm install -g @openai/codex
fi

# npm can exit 0 but leave the binary off PATH (e.g. prefix mismatch).
if ! command -v codex >/dev/null 2>&1; then
  echo "[warden] ERROR: Codex CLI installation failed" >&2
  exit 1
fi

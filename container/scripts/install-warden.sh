#!/usr/bin/env bash
set -euo pipefail

# -------------------------------------------------------------------
# Install Warden terminal management scripts to /usr/local/bin/.
#
# Copies scripts from the staging directory (shared/, claude/, codex/)
# to /usr/local/bin/ where they're accessible at runtime. Works with
# both the Dockerfile layout (subdirectories) and the devcontainer
# feature layout (flat, all scripts alongside this file).
# -------------------------------------------------------------------

SCRIPTS_DIR="$(cd "$(dirname "$0")" && pwd)"

# -------------------------------------------------------------------
# Detect layout: subdirectories (Dockerfile) or flat (devcontainer).
# -------------------------------------------------------------------
if [ -d "${SCRIPTS_DIR}/shared" ]; then
  # Dockerfile layout: scripts are in subdirectories.
  for dir in shared claude codex; do
    if [ -d "${SCRIPTS_DIR}/${dir}" ]; then
      for script in "${SCRIPTS_DIR}/${dir}"/*.sh; do
        [ -f "$script" ] || continue
        cp "$script" "/usr/local/bin/$(basename "$script")"
        chmod +x "/usr/local/bin/$(basename "$script")"
      done
    fi
  done
else
  # Devcontainer feature layout: all scripts flat alongside this file.
  # Use glob to avoid a hardcoded list going stale when scripts are
  # added. Excludes install-*.sh since those are build-time only.
  for script in "${SCRIPTS_DIR}"/*.sh; do
    [ -f "$script" ] || continue
    case "$(basename "$script")" in
      install-*.sh) continue ;;
    esac
    cp "$script" "/usr/local/bin/$(basename "$script")"
    chmod +x "/usr/local/bin/$(basename "$script")"
  done
fi

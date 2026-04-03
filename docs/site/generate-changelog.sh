#!/usr/bin/env bash
# Copies CHANGELOG.md into the docs site with Starlight frontmatter prepended
# and the duplicate "# Changelog" heading removed.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
OUTPUT="${SCRIPT_DIR}/src/content/docs/changelog.md"

cat > "${OUTPUT}" <<'FRONTMATTER'
---
title: Changelog
description: Release history for Warden.
editUrl: false
---

FRONTMATTER

tail -n +2 "${PROJECT_ROOT}/CHANGELOG.md" >> "${OUTPUT}"

echo "Generated changelog at ${OUTPUT}"

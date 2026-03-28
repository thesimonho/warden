#!/usr/bin/env bash
# Copies CONTRIBUTING.md into the docs site with Starlight frontmatter.
# Output is gitignored — regenerated at build time.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_FILE="${SCRIPT_DIR}/src/content/docs/contributing.md"

echo "Generating contributing page from CONTRIBUTING.md..."

# Read the source file, skip the "# Contributing" H1 heading (Starlight uses the frontmatter title),
# and convert absolute docs site URLs to relative paths for internal navigation.
content=$(tail -n +2 "${PROJECT_ROOT}/CONTRIBUTING.md" | \
  sed 's|https://thesimonho.github.io/warden/|/warden/|g')

cat > "${OUTPUT_FILE}" <<EOF
---
title: Contributing
description: How to contribute to Warden — development setup, coding guidelines, and submission process.
---
${content}
EOF

echo "  Written to ${OUTPUT_FILE}"

#!/usr/bin/env bash
# Generates Go package documentation as Markdown for the docs site.
# Output goes to website/src/content/docs/reference/go/ (gitignored).
# Each file includes a link to the pkg.go.dev page.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"
OUTPUT_DIR="${SCRIPT_DIR}/src/content/docs/reference/go"

MODULE_PATH="github.com/thesimonho/warden"

PACKAGES=(
  "."
  "access"
  "agent"
  "api"
  "client"
  "db"
  "engine"
  "eventbus"
  "runtime"
  "service"
)

cd "${PROJECT_ROOT}"

for pkg in "${PACKAGES[@]}"; do
  if [ "${pkg}" = "." ]; then
    pkg_name="warden"
    import_path="${MODULE_PATH}"
    pkg_dir="."
  else
    pkg_name="${pkg}"
    import_path="${MODULE_PATH}/${pkg}"
    pkg_dir="./${pkg}"
  fi

  output_file="${OUTPUT_DIR}/${pkg_name}.md"

  echo "Generating docs for ${import_path}..."

  # Generate markdown to stdout
  doc_content=$(gomarkdoc "${pkg_dir}" 2>/dev/null || true)

  if [ -z "${doc_content}" ]; then
    echo "  Skipping ${pkg_name} (no exportable content)"
    continue
  fi

  cat > "${output_file}" <<EOF
---
title: "${pkg_name}"
description: "Go package documentation for ${import_path}"
editUrl: false
---

> [View on pkg.go.dev](https://pkg.go.dev/${import_path})

${doc_content}
EOF

  echo "  Written to ${output_file}"
done

echo "Done. Generated Go reference docs in ${OUTPUT_DIR}"
